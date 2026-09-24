# Security Policy

## Supported versions
Only the latest released minor version receives security fixes.

## Reporting a vulnerability
Please **do not** open a public issue. Use GitHub's private vulnerability reporting
("Security" tab → "Report a vulnerability") on this repository.
You can expect an acknowledgement within a few days.

## Design notes relevant to security
- The exporter is read-only and needs only a vCenter/ESXi account with the built-in **Read-only** role.
- Credentials are accepted only via environment variables or a password file, never as command-line flags or URL userinfo, and are never logged.
- TLS verification is on by default. Disabling it requires the explicit `--vsphere.insecure-skip-verify` flag and logs a warning.
- Serial numbers / service tags are not exported unless `--export-serial` is set.
- `/metrics` has no authentication; bind it to a private interface or put it behind a reverse proxy / network policy.
- The container image is distroless, runs as a non-root user and contains only the static binary.
