# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.1] - 2026-09-24

### Fixed
- Release workflow: GitHub release notes are now taken from this changelog (they were empty in 0.1.0).
- Release workflow: release notes are written outside the checkout so GoReleaser sees a clean tree.

No changes to the exporter binary's behavior compared to 0.1.0.

## [0.1.0] - 2026-09-24

### Added
- Initial implementation: ESXi host hardware inventory (machine, BIOS, CPU, memory, NICs, disks, HBAs, ESXi product) as Prometheus info metrics.
- Background refresh with a cached snapshot; `/metrics` never waits for vCenter.
- Opt-in serial number export (`--export-serial`).
- Host include/exclude filters, CA file, explicit insecure-skip-verify opt-in, password file.
- Docker image (linux/amd64, linux/arm64), Grafana dashboard and example scrape configs.

[Unreleased]: https://github.com/redhatua/vsphere-hardware-exporter/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/redhatua/vsphere-hardware-exporter/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/redhatua/vsphere-hardware-exporter/releases/tag/v0.1.0
