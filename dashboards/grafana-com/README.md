# vSphere hardware inventory

Static **hardware inventory of your VMware ESXi hosts** as Grafana tables: what each host is, what is inside it, and which ESXi build it runs.

Popular vSphere integrations export performance counters. This dashboard covers the part they leave out, and is built for questions like:

- Which hosts are still on an old ESXi version or build?
- Which servers have an old BIOS, and what CPU, sockets, cores and RAM do they have?
- Which NICs are 1 GbE in a 10 GbE estate, or have their link down?
- What disks and LUNs does each host have: vendor, model, capacity, SSD or HDD, local or remote?
- Which storage HBAs and drivers are in use?

## Panels

| Panel | Contents |
|---|---|
| Summary | Number of hosts, total CPU cores, total memory, exporter health |
| Host hardware | Host, cluster, vendor, model, CPU, sockets, cores, threads, memory, ESXi version and build, maintenance mode, BIOS version and date, datacenter, vCenter |
| Physical NICs | Device, driver, MAC address, link speed (shows "link down" when the link is down) |
| Disks and LUNs | Canonical name, vendor, model, capacity, SSD, local |
| Storage HBAs | Device, model, driver, type |

Variables: **Datasource**, **vCenter**, **Datacenter**, **Cluster**, **Host**. All tables are filterable and sortable.

## Requirements

1. A Prometheus-compatible datasource (Prometheus, VictoriaMetrics, Mimir, ...).
2. [**vsphere-hardware-exporter**](https://github.com/redhatua/vsphere-hardware-exporter) running against your vCenter or ESXi host and scraped by that datasource. It is read-only, needs only a *Read-only* vSphere account, and refreshes hardware data in the background, so a slow vCenter never blocks a scrape.

```bash
docker run -d -p 9877:9877 --user "$(id -u):$(id -g)" \
  -v "$PWD/vsphere_password.txt:/run/secrets/vsphere_password:ro" \
  -e VSPHERE_URL=https://vcenter.example.com \
  -e VSPHERE_USERNAME=readonly@vsphere.local \
  -e VSPHERE_PASSWORD_FILE=/run/secrets/vsphere_password \
  ghcr.io/redhatua/vsphere-hardware-exporter:latest
```

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: vsphere-hardware
    scrape_interval: 5m
    static_configs:
      - targets: ["vsphere-hardware-exporter:9877"]
```

Metrics used: `vsphere_host_hw_info`, `vsphere_host_cpu_*`, `vsphere_host_memory_bytes`, `vsphere_host_in_maintenance_mode`, `vsphere_host_nic_info`, `vsphere_host_nic_speed_mbps`, `vsphere_host_disk_info`, `vsphere_host_disk_capacity_bytes`, `vsphere_host_hba_info`, `vsphere_hw_exporter_up`.

Serial numbers are **not** exported unless you enable `--export-serial`; this dashboard does not need them.

## Compatibility

Tested with Grafana 11.6 and 13.2 (import and file provisioning), using the exporter against the govmomi vCenter simulator. The exporter targets vSphere 6.5 to 8.x.

## Links

- Exporter, docs and issues: https://github.com/redhatua/vsphere-hardware-exporter
- Dashboard source: `dashboards/vsphere-hardware.json` in that repository
