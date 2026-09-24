package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"time"

	"github.com/vmware/govmomi/license"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// Source describes a vCenter or ESXi endpoint to read from.
type Source struct {
	URL      *url.URL // credentials are taken from Username/Password, never from the URL
	Username string
	Password string
	CAFile   string
	Insecure bool

	Include *regexp.Regexp // optional host name filters
	Exclude *regexp.Regexp

	Logger *slog.Logger
}

var hostProps = []string{
	"name", "parent", "runtime",
	"summary.hardware", "summary.config.product",
	"hardware.biosInfo",
	"config.network.pnic", "config.storageDevice",
}

// Fetch logs in, reads all hosts and logs out. The returned Snapshot is complete or an error is returned.
func (s *Source) Fetch(ctx context.Context) (*Snapshot, error) {
	c, logout, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer logout()

	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("create host view: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var hosts []mo.HostSystem
	if err := v.Retrieve(ctx, []string{"HostSystem"}, hostProps, &hosts); err != nil {
		return nil, fmt.Errorf("retrieve hosts: %w", err)
	}

	hosts = s.filter(hosts)

	loc, err := resolveLocations(ctx, property.DefaultCollector(c), hosts)
	if err != nil {
		return nil, fmt.Errorf("resolve datacenter/cluster: %w", err)
	}
	licenses := s.licenses(ctx, c)

	snap := &Snapshot{VCenter: s.URL.Hostname(), FetchedAt: time.Now()}
	for _, h := range hosts {
		out := hostFromMO(h)
		l := loc[h.Reference()]
		out.Datacenter, out.Cluster = l.datacenter, l.cluster
		out.License = licenses[out.UUID]
		snap.Hosts = append(snap.Hosts, out)
	}
	sort.Slice(snap.Hosts, func(i, j int) bool { return snap.Hosts[i].Name < snap.Hosts[j].Name })
	return snap, nil
}

func (s *Source) connect(ctx context.Context) (*vim25.Client, func(), error) {
	u := *s.URL
	u.User = nil
	sc := soap.NewClient(&u, s.Insecure)
	if s.CAFile != "" {
		if err := sc.SetRootCAs(s.CAFile); err != nil {
			return nil, nil, fmt.Errorf("load CA file: %w", err)
		}
	}
	c, err := vim25.NewClient(ctx, sc)
	if err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	sm := session.NewManager(c)
	if err := sm.Login(ctx, url.UserPassword(s.Username, s.Password)); err != nil {
		return nil, nil, fmt.Errorf("login: %w", err)
	}
	logout := func() { _ = sm.Logout(context.WithoutCancel(ctx)) }
	return c, logout, nil
}

func (s *Source) filter(in []mo.HostSystem) []mo.HostSystem {
	out := in[:0]
	for _, h := range in {
		if s.Include != nil && !s.Include.MatchString(h.Name) {
			continue
		}
		if s.Exclude != nil && s.Exclude.MatchString(h.Name) {
			continue
		}
		out = append(out, h)
	}
	return out
}

// licenses returns license names keyed by host hardware UUID. It is best effort:
// read-only accounts or standalone ESXi may not be allowed to query assignments.
func (s *Source) licenses(ctx context.Context, c *vim25.Client) map[string]string {
	out := map[string]string{}
	am, err := license.NewManager(c).AssignmentManager(ctx)
	if err == nil {
		var as []types.LicenseAssignmentManagerLicenseAssignment
		if as, err = am.QueryAssigned(ctx, ""); err == nil {
			for _, a := range as {
				out[a.EntityId] = a.AssignedLicense.Name
			}
			return out
		}
	}
	if s.Logger != nil {
		s.Logger.Warn("license information not readable; license_info will be omitted", "error", err)
	}
	return out
}

type location struct{ datacenter, cluster string }

// resolveLocations walks parent references upwards level by level, using one
// round trip per level regardless of the number of hosts.
func resolveLocations(ctx context.Context, pc *property.Collector, hosts []mo.HostSystem) (map[types.ManagedObjectReference]location, error) {
	type walker struct {
		cur types.ManagedObjectReference
		loc location
	}
	res := make(map[types.ManagedObjectReference]location, len(hosts))
	active := map[types.ManagedObjectReference]*walker{}
	for _, h := range hosts {
		if h.Parent != nil {
			active[h.Reference()] = &walker{cur: *h.Parent}
		} else {
			res[h.Reference()] = location{}
		}
	}

	const maxDepth = 32
	for depth := 0; len(active) > 0; depth++ {
		if depth > maxDepth {
			return nil, fmt.Errorf("inventory hierarchy deeper than %d levels", maxDepth)
		}
		uniq := map[types.ManagedObjectReference]struct{}{}
		for _, w := range active {
			uniq[w.cur] = struct{}{}
		}
		refs := make([]types.ManagedObjectReference, 0, len(uniq))
		for r := range uniq {
			refs = append(refs, r)
		}
		var ents []mo.ManagedEntity
		if err := pc.Retrieve(ctx, refs, []string{"name", "parent"}, &ents); err != nil {
			return nil, err
		}
		byRef := make(map[types.ManagedObjectReference]mo.ManagedEntity, len(ents))
		for _, e := range ents {
			byRef[e.Reference()] = e
		}
		for h, w := range active {
			e, ok := byRef[w.cur]
			if !ok {
				res[h] = w.loc
				delete(active, h)
				continue
			}
			switch w.cur.Type {
			case "ClusterComputeResource":
				w.loc.cluster = e.Name
			case "Datacenter":
				w.loc.datacenter = e.Name
			}
			if e.Parent == nil || w.cur.Type == "Datacenter" {
				res[h] = w.loc
				delete(active, h)
				continue
			}
			w.cur = *e.Parent
		}
	}
	return res, nil
}
