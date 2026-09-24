package collector

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/redhatua/vsphere-hardware-exporter/internal/inventory"
)

type fakeSource struct {
	snap  *inventory.Snapshot
	state State
}

func (f fakeSource) Get() (*inventory.Snapshot, State) { return f.snap, f.state }

func testHost() inventory.Host {
	return inventory.Host{
		Name: "esx01.example.com", MoID: "host-10", Datacenter: "dc1", Cluster: "cl1",
		Connected: true, PoweredOn: true,
		Vendor: "ExampleCorp", Model: "EX-1000", Serial: "SN0001", UUID: "uuid-1",
		BIOSVersion: "1.2.3", BIOSDate: "2024-01-02",
		CPUModel: "Example CPU @ 2.0GHz", CPUSockets: 2, CPUCores: 32, CPUThreads: 64, CPUMHz: 2000,
		MemoryBytes: 274877906944,
		ESXiVersion: "8.0.2", ESXiBuild: "12345678", License: "Example Edition",
		NICs:  []inventory.NIC{{Device: "vmnic0", Driver: "ixgben", MAC: "00:00:5e:00:53:01", SpeedMbps: 10000, LinkUp: true}, {Device: "vmnic1", Driver: "ixgben", MAC: "00:00:5e:00:53:02"}},
		Disks: []inventory.Disk{{CanonicalName: "naa.1", Vendor: "EX", Model: "DISK", CapacityBytes: 1000, SSD: true, Local: true}},
		HBAs:  []inventory.HBA{{Device: "vmhba0", Model: "SAS HBA", Driver: "lsi_msgpt3", Type: "block"}},
	}
}

func collect(t *testing.T, opts Options, src Source, expected string, names ...string) {
	t.Helper()
	c := New(src, opts)
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), names...); err != nil {
		t.Fatal(err)
	}
}

func snapshotSource(h inventory.Host) fakeSource {
	return fakeSource{
		snap:  &inventory.Snapshot{VCenter: "vc.example.com", Hosts: []inventory.Host{h}},
		state: State{Up: true, LastSuccess: time.Unix(1700000000, 0), Duration: 1500 * time.Millisecond},
	}
}

func TestHardwareInfo(t *testing.T) {
	collect(t, Options{}, snapshotSource(testHost()), `
# HELP vsphere_host_hw_info Static hardware and ESXi product information of an ESXi host (value is always 1).
# TYPE vsphere_host_hw_info gauge
vsphere_host_hw_info{bios_date="2024-01-02",bios_version="1.2.3",cluster="cl1",cpu_model="Example CPU @ 2.0GHz",datacenter="dc1",esxi_build="12345678",esxi_version="8.0.2",host="esx01.example.com",model="EX-1000",vcenter="vc.example.com",vendor="ExampleCorp"} 1
`, "vsphere_host_hw_info")
}

func TestSerialOptIn(t *testing.T) {
	const want = `
# HELP vsphere_host_serial_info Serial number / service tag of an ESXi host (value is always 1). Only exported with --export-serial.
# TYPE vsphere_host_serial_info gauge
vsphere_host_serial_info{cluster="cl1",datacenter="dc1",host="esx01.example.com",serial="SN0001",vcenter="vc.example.com"} 1
`
	collect(t, Options{ExportSerial: true}, snapshotSource(testHost()), want, "vsphere_host_serial_info")
	collect(t, Options{}, snapshotSource(testHost()), "", "vsphere_host_serial_info")
}

func TestCPUAndMemory(t *testing.T) {
	collect(t, Options{}, snapshotSource(testHost()), `
# HELP vsphere_host_cpu_cores Number of physical CPU cores.
# TYPE vsphere_host_cpu_cores gauge
vsphere_host_cpu_cores{cluster="cl1",datacenter="dc1",host="esx01.example.com",vcenter="vc.example.com"} 32
# HELP vsphere_host_cpu_threads Number of logical CPU threads.
# TYPE vsphere_host_cpu_threads gauge
vsphere_host_cpu_threads{cluster="cl1",datacenter="dc1",host="esx01.example.com",vcenter="vc.example.com"} 64
# HELP vsphere_host_cpu_sockets Number of physical CPU packages.
# TYPE vsphere_host_cpu_sockets gauge
vsphere_host_cpu_sockets{cluster="cl1",datacenter="dc1",host="esx01.example.com",vcenter="vc.example.com"} 2
# HELP vsphere_host_memory_bytes Installed physical memory in bytes.
# TYPE vsphere_host_memory_bytes gauge
vsphere_host_memory_bytes{cluster="cl1",datacenter="dc1",host="esx01.example.com",vcenter="vc.example.com"} 2.74877906944e+11
`, "vsphere_host_cpu_cores", "vsphere_host_cpu_threads", "vsphere_host_cpu_sockets", "vsphere_host_memory_bytes")
}

func TestStateGauges(t *testing.T) {
	h := testHost()
	h.InMaintenance = true
	h.Connected = false
	collect(t, Options{}, snapshotSource(h), `
# HELP vsphere_host_connected 1 if the host is connected to vCenter.
# TYPE vsphere_host_connected gauge
vsphere_host_connected{cluster="cl1",datacenter="dc1",host="esx01.example.com",vcenter="vc.example.com"} 0
# HELP vsphere_host_in_maintenance_mode 1 if the host is in maintenance mode.
# TYPE vsphere_host_in_maintenance_mode gauge
vsphere_host_in_maintenance_mode{cluster="cl1",datacenter="dc1",host="esx01.example.com",vcenter="vc.example.com"} 1
`, "vsphere_host_connected", "vsphere_host_in_maintenance_mode")
}

func TestNICs(t *testing.T) {
	collect(t, Options{}, snapshotSource(testHost()), `
# HELP vsphere_host_nic_info Physical NIC information (value is always 1).
# TYPE vsphere_host_nic_info gauge
vsphere_host_nic_info{cluster="cl1",datacenter="dc1",device="vmnic0",driver="ixgben",host="esx01.example.com",mac="00:00:5e:00:53:01",vcenter="vc.example.com"} 1
vsphere_host_nic_info{cluster="cl1",datacenter="dc1",device="vmnic1",driver="ixgben",host="esx01.example.com",mac="00:00:5e:00:53:02",vcenter="vc.example.com"} 1
# HELP vsphere_host_nic_speed_mbps Negotiated link speed in Mbps, 0 if the link is down.
# TYPE vsphere_host_nic_speed_mbps gauge
vsphere_host_nic_speed_mbps{cluster="cl1",datacenter="dc1",device="vmnic0",host="esx01.example.com",vcenter="vc.example.com"} 10000
vsphere_host_nic_speed_mbps{cluster="cl1",datacenter="dc1",device="vmnic1",host="esx01.example.com",vcenter="vc.example.com"} 0
`, "vsphere_host_nic_info", "vsphere_host_nic_speed_mbps")
}

func TestDisksAndHBAs(t *testing.T) {
	collect(t, Options{}, snapshotSource(testHost()), `
# HELP vsphere_host_disk_info SCSI disk information (value is always 1).
# TYPE vsphere_host_disk_info gauge
vsphere_host_disk_info{canonical_name="naa.1",cluster="cl1",datacenter="dc1",host="esx01.example.com",local="true",model="DISK",ssd="true",vcenter="vc.example.com",vendor="EX"} 1
# HELP vsphere_host_disk_capacity_bytes Disk capacity in bytes.
# TYPE vsphere_host_disk_capacity_bytes gauge
vsphere_host_disk_capacity_bytes{canonical_name="naa.1",cluster="cl1",datacenter="dc1",host="esx01.example.com",vcenter="vc.example.com"} 1000
# HELP vsphere_host_hba_info Storage HBA information (value is always 1).
# TYPE vsphere_host_hba_info gauge
vsphere_host_hba_info{cluster="cl1",datacenter="dc1",device="vmhba0",driver="lsi_msgpt3",host="esx01.example.com",model="SAS HBA",type="block",vcenter="vc.example.com"} 1
`, "vsphere_host_disk_info", "vsphere_host_disk_capacity_bytes", "vsphere_host_hba_info")
}

func TestLicenseOmittedWhenEmpty(t *testing.T) {
	h := testHost()
	h.License = ""
	collect(t, Options{}, snapshotSource(h), "", "vsphere_host_license_info")
}

func TestExporterHealthNoSnapshot(t *testing.T) {
	collect(t, Options{}, fakeSource{state: State{Up: false}}, `
# HELP vsphere_hw_exporter_up 1 if the last refresh of the vSphere inventory succeeded.
# TYPE vsphere_hw_exporter_up gauge
vsphere_hw_exporter_up 0
`, "vsphere_hw_exporter_up")
}

func TestExporterHealth(t *testing.T) {
	collect(t, Options{}, snapshotSource(testHost()), `
# HELP vsphere_hw_exporter_last_success_timestamp_seconds Unix time of the last successful refresh.
# TYPE vsphere_hw_exporter_last_success_timestamp_seconds gauge
vsphere_hw_exporter_last_success_timestamp_seconds 1.7e+09
# HELP vsphere_hw_exporter_scrape_duration_seconds Duration of the last refresh in seconds.
# TYPE vsphere_hw_exporter_scrape_duration_seconds gauge
vsphere_hw_exporter_scrape_duration_seconds 1.5
# HELP vsphere_hw_exporter_up 1 if the last refresh of the vSphere inventory succeeded.
# TYPE vsphere_hw_exporter_up gauge
vsphere_hw_exporter_up 1
`, "vsphere_hw_exporter_up", "vsphere_hw_exporter_last_success_timestamp_seconds", "vsphere_hw_exporter_scrape_duration_seconds")
}

func TestLintPassesMetadata(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	if err := reg.Register(New(snapshotSource(testHost()), Options{ExportSerial: true})); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Gather(); err != nil {
		t.Fatal(err)
	}
}
