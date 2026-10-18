# Maintaining the grafana.com listing

Dashboard: https://grafana.com/grafana/dashboards/25818

| File | Purpose |
|---|---|
| `vsphere-hardware.json` | The file uploaded to grafana.com (Grafana "share externally" export) |
| `README.md` | Listing README (paste into the dashboard's README field) |
| `short-description.txt` | Listing short description |

`../vsphere-hardware.json` is the source of truth and is what people provision from. The grafana.com copy is derived
from it, because grafana.com only accepts JSON exported by Grafana itself (Classic model, "Share dashboard with another
instance" on); a hand-written file is rejected with "Old dashboard JSON format".

To refresh it after changing `../vsphere-hardware.json`:

1. Load the main dashboard into **Grafana 11 or 12** (Grafana 13 exports the v2 schema by default, which is not the
   Classic model).
2. *Export → Export as JSON*, switch on *Share dashboard with another instance*, download.
3. Rename the datasource input `DS_PROM` (or whatever your datasource is called) to `DS_PROMETHEUS`, set its label to
   `Prometheus`, and make sure `"id": null`. Do not commit any local datasource names or URLs.
4. Replace `vsphere-hardware.json` here, run `go test ./dashboards` (it checks the copy still matches the main
   dashboard), then upload it to grafana.com as a **new revision**. grafana.com does not pull from GitHub.
