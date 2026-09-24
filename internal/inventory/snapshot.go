// Package inventory fetches ESXi host hardware from vSphere into plain structs.
package inventory

import "time"

// Snapshot is one complete, immutable view of the inventory.
type Snapshot struct {
	VCenter   string
	FetchedAt time.Time
	Hosts     []Host
}

// Host is the static hardware description of one ESXi host.
type Host struct {
	Name       string
	MoID       string
	Datacenter string
	Cluster    string // empty for standalone hosts

	Connected     bool
	PoweredOn     bool
	InMaintenance bool

	Vendor      string
	Model       string
	Serial      string
	UUID        string
	BIOSVersion string
	BIOSDate    string

	CPUModel    string
	CPUSockets  int
	CPUCores    int
	CPUThreads  int
	CPUMHz      int
	MemoryBytes int64

	ESXiVersion string
	ESXiBuild   string
	License     string // empty when not readable

	NICs  []NIC
	Disks []Disk
	HBAs  []HBA
}

// NIC is a physical network adapter.
type NIC struct {
	Device    string
	Driver    string
	MAC       string
	SpeedMbps int // 0 when the link is down
	LinkUp    bool
}

// Disk is a SCSI disk LUN.
type Disk struct {
	CanonicalName string
	Vendor        string
	Model         string
	CapacityBytes int64
	SSD           bool
	Local         bool
}

// HBA is a storage host bus adapter.
type HBA struct {
	Device string
	Model  string
	Driver string
	Type   string // e.g. fibreChannel, iscsi, block, parallelScsi
}
