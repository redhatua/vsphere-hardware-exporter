package inventory

import (
	"strings"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// serialKeys lists identifyingInfo keys in order of preference.
var serialKeys = []string{"ServiceTag", "SerialNumberTag"}

// hostFromMO maps the fetched properties of one HostSystem to a Host.
// Nil sections (e.g. hardware of a disconnected host) are tolerated.
func hostFromMO(h mo.HostSystem) Host {
	out := Host{Name: h.Name, MoID: h.Reference().Value}

	out.Connected = h.Runtime.ConnectionState == types.HostSystemConnectionStateConnected
	out.PoweredOn = h.Runtime.PowerState == types.HostSystemPowerStatePoweredOn
	out.InMaintenance = h.Runtime.InMaintenanceMode

	if hw := h.Summary.Hardware; hw != nil {
		out.Vendor = hw.Vendor
		out.Model = hw.Model
		out.UUID = hw.Uuid
		out.CPUModel = strings.TrimSpace(hw.CpuModel)
		out.CPUSockets = int(hw.NumCpuPkgs)
		out.CPUCores = int(hw.NumCpuCores)
		out.CPUThreads = int(hw.NumCpuThreads)
		out.CPUMHz = int(hw.CpuMhz)
		out.MemoryBytes = hw.MemorySize
		out.Serial = serial(hw.OtherIdentifyingInfo)
	}
	if p := h.Summary.Config.Product; p != nil {
		out.ESXiVersion = p.Version
		out.ESXiBuild = p.Build
	}
	if h.Hardware != nil && h.Hardware.BiosInfo != nil {
		out.BIOSVersion = strings.TrimSpace(h.Hardware.BiosInfo.BiosVersion)
		if d := h.Hardware.BiosInfo.ReleaseDate; d != nil {
			out.BIOSDate = d.UTC().Format("2006-01-02")
		}
	}

	if h.Config != nil {
		if n := h.Config.Network; n != nil {
			for _, p := range n.Pnic {
				nic := NIC{Device: p.Device, Driver: p.Driver, MAC: p.Mac}
				if p.LinkSpeed != nil && p.LinkSpeed.SpeedMb > 0 {
					nic.SpeedMbps = int(p.LinkSpeed.SpeedMb)
					nic.LinkUp = true
				}
				out.NICs = append(out.NICs, nic)
			}
		}
		if s := h.Config.StorageDevice; s != nil {
			for _, l := range s.ScsiLun {
				if d, ok := l.(*types.HostScsiDisk); ok {
					out.Disks = append(out.Disks, diskFrom(d))
				}
			}
			for _, b := range s.HostBusAdapter {
				base := b.GetHostHostBusAdapter()
				out.HBAs = append(out.HBAs, HBA{
					Device: base.Device,
					Model:  strings.TrimSpace(base.Model),
					Driver: base.Driver,
					Type:   hbaType(b),
				})
			}
		}
	}
	return out
}

func serial(info []types.HostSystemIdentificationInfo) string {
	for _, key := range serialKeys {
		for _, i := range info {
			if i.IdentifierType == nil || i.IdentifierValue == "" {
				continue
			}
			if i.IdentifierType.GetElementDescription().Key == key {
				return i.IdentifierValue
			}
		}
	}
	return ""
}

func diskFrom(d *types.HostScsiDisk) Disk {
	return Disk{
		CanonicalName: d.CanonicalName,
		Vendor:        strings.TrimSpace(d.Vendor),
		Model:         strings.TrimSpace(d.Model),
		CapacityBytes: d.Capacity.Block * int64(d.Capacity.BlockSize),
		SSD:           d.Ssd != nil && *d.Ssd,
		Local:         d.LocalDisk != nil && *d.LocalDisk,
	}
}

func hbaType(b types.BaseHostHostBusAdapter) string {
	switch b.(type) {
	case *types.HostBlockHba:
		return "block"
	case *types.HostFibreChannelHba:
		return "fibreChannel"
	case *types.HostFibreChannelOverEthernetHba:
		return "fcoe"
	case *types.HostInternetScsiHba:
		return "iscsi"
	case *types.HostParallelScsiHba:
		return "parallelScsi"
	case *types.HostSerialAttachedHba:
		return "sas"
	case *types.HostTcpHba:
		return "tcp"
	case *types.HostPcieHba:
		return "pcie"
	default:
		return "other"
	}
}
