# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Initial implementation: ESXi host hardware inventory (machine, BIOS, CPU, memory, NICs, disks, HBAs, ESXi product) as Prometheus info metrics.
- Background refresh with a cached snapshot; `/metrics` never waits for vCenter.
- Opt-in serial number export (`--export-serial`).
- Host include/exclude filters, CA file, explicit insecure-skip-verify opt-in, password file.
- Docker image (linux/amd64, linux/arm64), Grafana dashboard and example scrape configs.
