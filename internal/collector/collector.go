// Package collector turns an inventory Snapshot into Prometheus metrics.
package collector

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/redhatua/vsphere-hardware-exporter/internal/inventory"
)

// Source provides the latest cached snapshot and refresh state.
type Source interface {
	Get() (*inventory.Snapshot, State)
}

// Options controls optional metrics.
type Options struct {
	ExportSerial bool
}

// Collector implements prometheus.Collector on top of a Source.
type Collector struct {
	src  Source
	opts Options

	hwInfo, uuidInfo, serialInfo, licenseInfo *prometheus.Desc
	cpuSockets, cpuCores, cpuThreads, cpuMHz  *prometheus.Desc
	memory                                    *prometheus.Desc
	connected, poweredOn, maintenance         *prometheus.Desc
	nicInfo, nicSpeed                         *prometheus.Desc
	diskInfo, diskCapacity, hbaInfo           *prometheus.Desc
	up, lastSuccess, duration                 *prometheus.Desc
}

var hostLabels = []string{"vcenter", "datacenter", "cluster", "host"}

func desc(name, help string, extra ...string) *prometheus.Desc {
	return prometheus.NewDesc("vsphere_host_"+name, help, append(append([]string{}, hostLabels...), extra...), nil)
}

func exporterDesc(name, help string) *prometheus.Desc {
	return prometheus.NewDesc("vsphere_hw_exporter_"+name, help, nil, nil)
}

// New returns a Collector reading from src.
func New(src Source, opts Options) *Collector {
	return &Collector{
		src: src, opts: opts,
		hwInfo:       desc("hw_info", "Static hardware and ESXi product information of an ESXi host (value is always 1).", "vendor", "model", "bios_version", "bios_date", "cpu_model", "esxi_version", "esxi_build"),
		uuidInfo:     desc("uuid_info", "Hardware UUID of an ESXi host (value is always 1).", "uuid"),
		serialInfo:   desc("serial_info", "Serial number / service tag of an ESXi host (value is always 1). Only exported with --export-serial.", "serial"),
		licenseInfo:  desc("license_info", "Assigned license of an ESXi host (value is always 1). Only exported when readable.", "license"),
		cpuSockets:   desc("cpu_sockets", "Number of physical CPU packages."),
		cpuCores:     desc("cpu_cores", "Number of physical CPU cores."),
		cpuThreads:   desc("cpu_threads", "Number of logical CPU threads."),
		cpuMHz:       desc("cpu_mhz", "CPU core frequency in MHz."),
		memory:       desc("memory_bytes", "Installed physical memory in bytes."),
		connected:    desc("connected", "1 if the host is connected to vCenter."),
		poweredOn:    desc("powered_on", "1 if the host is powered on."),
		maintenance:  desc("in_maintenance_mode", "1 if the host is in maintenance mode."),
		nicInfo:      desc("nic_info", "Physical NIC information (value is always 1).", "device", "driver", "mac"),
		nicSpeed:     desc("nic_speed_mbps", "Negotiated link speed in Mbps, 0 if the link is down.", "device"),
		diskInfo:     desc("disk_info", "SCSI disk information (value is always 1).", "canonical_name", "vendor", "model", "ssd", "local"),
		diskCapacity: desc("disk_capacity_bytes", "Disk capacity in bytes.", "canonical_name"),
		hbaInfo:      desc("hba_info", "Storage HBA information (value is always 1).", "device", "model", "driver", "type"),
		up:           exporterDesc("up", "1 if the last refresh of the vSphere inventory succeeded."),
		lastSuccess:  exporterDesc("last_success_timestamp_seconds", "Unix time of the last successful refresh."),
		duration:     exporterDesc("scrape_duration_seconds", "Duration of the last refresh in seconds."),
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		c.hwInfo, c.uuidInfo, c.serialInfo, c.licenseInfo, c.cpuSockets, c.cpuCores, c.cpuThreads, c.cpuMHz,
		c.memory, c.connected, c.poweredOn, c.maintenance, c.nicInfo, c.nicSpeed, c.diskInfo, c.diskCapacity,
		c.hbaInfo, c.up, c.lastSuccess, c.duration,
	} {
		ch <- d
	}
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	snap, st := c.src.Get()

	g := func(d *prometheus.Desc, v float64, lv ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, lv...)
	}
	g(c.up, b2f(st.Up))
	if !st.LastSuccess.IsZero() {
		g(c.lastSuccess, float64(st.LastSuccess.Unix()))
		g(c.duration, st.Duration.Seconds())
	}
	if snap == nil {
		return
	}

	for _, h := range snap.Hosts {
		base := []string{snap.VCenter, h.Datacenter, h.Cluster, h.Name}
		with := func(extra ...string) []string { return append(append([]string{}, base...), extra...) }

		g(c.hwInfo, 1, with(h.Vendor, h.Model, h.BIOSVersion, h.BIOSDate, h.CPUModel, h.ESXiVersion, h.ESXiBuild)...)
		if h.UUID != "" {
			g(c.uuidInfo, 1, with(h.UUID)...)
		}
		if c.opts.ExportSerial && h.Serial != "" {
			g(c.serialInfo, 1, with(h.Serial)...)
		}
		if h.License != "" {
			g(c.licenseInfo, 1, with(h.License)...)
		}
		g(c.cpuSockets, float64(h.CPUSockets), base...)
		g(c.cpuCores, float64(h.CPUCores), base...)
		g(c.cpuThreads, float64(h.CPUThreads), base...)
		g(c.cpuMHz, float64(h.CPUMHz), base...)
		g(c.memory, float64(h.MemoryBytes), base...)
		g(c.connected, b2f(h.Connected), base...)
		g(c.poweredOn, b2f(h.PoweredOn), base...)
		g(c.maintenance, b2f(h.InMaintenance), base...)

		for _, n := range h.NICs {
			g(c.nicInfo, 1, with(n.Device, n.Driver, n.MAC)...)
			g(c.nicSpeed, float64(n.SpeedMbps), with(n.Device)...)
		}
		for _, d := range h.Disks {
			g(c.diskInfo, 1, with(d.CanonicalName, d.Vendor, d.Model, strconv.FormatBool(d.SSD), strconv.FormatBool(d.Local))...)
			g(c.diskCapacity, float64(d.CapacityBytes), with(d.CanonicalName)...)
		}
		for _, a := range h.HBAs {
			g(c.hbaInfo, 1, with(a.Device, a.Model, a.Driver, a.Type)...)
		}
	}
}
