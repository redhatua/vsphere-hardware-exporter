package inventory

import (
	"testing"
	"time"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestHostFromMODisconnected(t *testing.T) {
	var h mo.HostSystem
	h.Name = "esx-down.example.com"
	h.Runtime.ConnectionState = types.HostSystemConnectionStateDisconnected
	h.Runtime.PowerState = types.HostSystemPowerStateUnknown

	got := hostFromMO(h) // must not panic on nil hardware/config
	if got.Connected || got.PoweredOn || got.Name != "esx-down.example.com" {
		t.Errorf("unexpected: %+v", got)
	}
	if len(got.NICs)+len(got.Disks)+len(got.HBAs) != 0 {
		t.Errorf("expected no devices: %+v", got)
	}
}

func TestSerialPreference(t *testing.T) {
	ed := func(k string) types.BaseElementDescription {
		return &types.ElementDescription{Key: k}
	}
	info := []types.HostSystemIdentificationInfo{
		{IdentifierValue: "ASSET-1", IdentifierType: ed("AssetTag")},
		{IdentifierValue: "SN-1", IdentifierType: ed("SerialNumberTag")},
		{IdentifierValue: "TAG-1", IdentifierType: ed("ServiceTag")},
	}
	if got := serial(info); got != "TAG-1" {
		t.Errorf("got %q, want ServiceTag", got)
	}
	if got := serial(info[:2]); got != "SN-1" {
		t.Errorf("got %q, want SerialNumberTag fallback", got)
	}
	if got := serial(info[:1]); got != "" {
		t.Errorf("AssetTag must not be used, got %q", got)
	}
}

func TestDiskFrom(t *testing.T) {
	yes := true
	d := diskFrom(&types.HostScsiDisk{
		ScsiLun:   types.ScsiLun{CanonicalName: "naa.1", Vendor: "EX      ", Model: "DISK  "},
		Capacity:  types.HostDiskDimensionsLba{BlockSize: 512, Block: 2000},
		Ssd:       &yes,
		LocalDisk: nil,
	})
	if d.Vendor != "EX" || d.Model != "DISK" || d.CapacityBytes != 1024000 || !d.SSD || d.Local {
		t.Errorf("got %+v", d)
	}
}

func TestNICLinkDownAndBIOSDate(t *testing.T) {
	var h mo.HostSystem
	rel := time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC)
	h.Hardware = &types.HostHardwareInfo{BiosInfo: &types.HostBIOSInfo{BiosVersion: " 1.0 ", ReleaseDate: &rel}}
	h.Config = &types.HostConfigInfo{Network: &types.HostNetworkInfo{Pnic: []types.PhysicalNic{
		{Device: "vmnic0", LinkSpeed: &types.PhysicalNicLinkInfo{SpeedMb: 25000}},
		{Device: "vmnic1"},
	}}}
	got := hostFromMO(h)
	if got.BIOSVersion != "1.0" || got.BIOSDate != "2024-03-04" {
		t.Errorf("bios = %q %q", got.BIOSVersion, got.BIOSDate)
	}
	if !got.NICs[0].LinkUp || got.NICs[0].SpeedMbps != 25000 || got.NICs[1].LinkUp || got.NICs[1].SpeedMbps != 0 {
		t.Errorf("nics = %+v", got.NICs)
	}
}
