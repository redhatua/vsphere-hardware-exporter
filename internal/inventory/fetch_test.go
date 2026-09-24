package inventory

import (
	"context"
	"net/url"
	"regexp"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func newSim(t *testing.T, model *simulator.Model) *Source {
	t.Helper()
	if err := model.Create(); err != nil {
		t.Fatal(err)
	}
	model.Service.Listen = &url.URL{User: url.UserPassword("ro-user", "s3cret-example")}
	t.Cleanup(model.Remove)
	s := model.Service.NewServer()
	t.Cleanup(s.Close)
	pass, _ := s.URL.User.Password()
	u, err := url.Parse(s.URL.String())
	if err != nil {
		t.Fatal(err)
	}
	u.User = nil
	return &Source{URL: u, Username: s.URL.User.Username(), Password: pass, Insecure: true}
}

func byName(snap *Snapshot) map[string]Host {
	m := map[string]Host{}
	for _, h := range snap.Hosts {
		m[h.Name] = h
	}
	return m
}

func TestFetchVCenter(t *testing.T) {
	src := newSim(t, simulator.VPX())
	snap, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.VCenter != "127.0.0.1" {
		t.Errorf("vcenter = %q", snap.VCenter)
	}
	hosts := byName(snap)
	if len(hosts) != 4 {
		t.Fatalf("want 4 hosts, got %d", len(hosts))
	}

	clustered := hosts["DC0_C0_H0"]
	if clustered.Datacenter != "DC0" || clustered.Cluster != "DC0_C0" {
		t.Errorf("clustered host location = %q/%q", clustered.Datacenter, clustered.Cluster)
	}
	standalone := hosts["DC0_H0"]
	if standalone.Datacenter != "DC0" || standalone.Cluster != "" {
		t.Errorf("standalone host location = %q/%q", standalone.Datacenter, standalone.Cluster)
	}

	h := clustered
	if !h.Connected || !h.PoweredOn || h.InMaintenance {
		t.Errorf("state = %v/%v/%v", h.Connected, h.PoweredOn, h.InMaintenance)
	}
	if h.Vendor == "" || h.Model == "" || h.UUID == "" || h.Serial == "" {
		t.Errorf("machine info incomplete: %+v", h)
	}
	if h.CPUModel == "" || h.CPUCores == 0 || h.CPUThreads == 0 || h.CPUSockets == 0 || h.CPUMHz == 0 || h.MemoryBytes == 0 {
		t.Errorf("cpu/memory incomplete: %+v", h)
	}
	if h.License == "" {
		t.Error("license not resolved; assignments are keyed by host MoID")
	}
	if h.ESXiVersion == "" || h.ESXiBuild == "" || h.BIOSVersion == "" || h.BIOSDate == "" {
		t.Errorf("product/bios incomplete: %+v", h)
	}
	if len(h.NICs) == 0 || h.NICs[0].Device != "vmnic0" || h.NICs[0].MAC == "" || !h.NICs[0].LinkUp || h.NICs[0].SpeedMbps == 0 {
		t.Errorf("nics = %+v", h.NICs)
	}
	if len(h.Disks) == 0 || h.Disks[0].CanonicalName == "" || h.Disks[0].CapacityBytes == 0 {
		t.Errorf("disks = %+v", h.Disks)
	}
	if len(h.HBAs) == 0 || h.HBAs[0].Device == "" || h.HBAs[0].Type == "" {
		t.Errorf("hbas = %+v", h.HBAs)
	}
}

func TestFetchStandaloneESXi(t *testing.T) {
	src := newSim(t, simulator.ESX())
	snap, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Hosts) != 1 {
		t.Fatalf("want 1 host, got %d", len(snap.Hosts))
	}
	if snap.Hosts[0].Cluster != "" || snap.Hosts[0].Datacenter == "" {
		t.Errorf("location = %q/%q", snap.Hosts[0].Datacenter, snap.Hosts[0].Cluster)
	}
}

func TestFetchFilters(t *testing.T) {
	src := newSim(t, simulator.VPX())
	src.Include = regexp.MustCompile(`^DC0_C0_`)
	src.Exclude = regexp.MustCompile(`_H2$`)
	snap, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	hosts := byName(snap)
	if len(hosts) != 2 || hosts["DC0_C0_H0"].Name == "" || hosts["DC0_C0_H1"].Name == "" {
		t.Errorf("filtered hosts = %v", hosts)
	}
}

func TestFetchBadPassword(t *testing.T) {
	src := newSim(t, simulator.VPX())
	src.Password = "wrong-password"
	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected login error")
	}
	if regexp.MustCompile("wrong-password").MatchString(err.Error()) {
		t.Errorf("error leaks password: %v", err)
	}
}

func TestFetchUnreachable(t *testing.T) {
	u, _ := url.Parse("https://192.0.2.1:1/sdk")
	ctx, cancel := context.WithTimeout(context.Background(), 200e6)
	defer cancel()
	if _, err := (&Source{URL: u, Username: "u", Password: "p"}).Fetch(ctx); err == nil {
		t.Fatal("expected error")
	}
}
