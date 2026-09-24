# Design

Read-only Prometheus exporter for VMware ESXi host hardware inventory.

## Refresh model
A background loop logs in, fetches all hosts with one PropertyCollector call,
logs out, and atomically swaps the snapshot. `/metrics` only reads the cache.
On failure the last good snapshot is kept and `vsphere_hw_exporter_up` is 0.
`/healthz` is liveness; `/ready` is 200 after the first successful refresh.

## Layout
- `internal/config` flags/env parsing
- `internal/inventory` govmomi fetch -> plain `Snapshot` structs
- `internal/collector` `Snapshot` -> const metrics (pure, table-tested)
- `internal/cache` snapshot holder and refresh loop

## Metrics
Common labels: `vcenter, datacenter, cluster, host`.
- `vsphere_host_hw_info{vendor,model,bios_version,bios_date,cpu_model,esxi_version,esxi_build}`
- `vsphere_host_uuid_info{uuid}`, `vsphere_host_serial_info{serial}` (opt-in)
- `vsphere_host_license_info{license}` (best effort, vCenter only)
- `vsphere_host_cpu_{sockets,cores,threads}`, `vsphere_host_cpu_mhz`, `vsphere_host_memory_bytes`
- `vsphere_host_connected`, `vsphere_host_powered_on`, `vsphere_host_in_maintenance_mode`
- `vsphere_host_nic_info{device,driver,mac}`, `vsphere_host_nic_speed_mbps`
- `vsphere_host_disk_info{canonical_name,vendor,model,ssd,local}`, `vsphere_host_disk_capacity_bytes`
- `vsphere_host_hba_info{device,model,driver,type}`
- `vsphere_hw_exporter_{scrape_duration_seconds,last_success_timestamp_seconds,up}`

## Decisions
Apache-2.0; one vCenter target per process; credentials only via env/file.
