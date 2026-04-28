# simsexam Testing And Deployment Policy

This document defines the current testing workflow, branch policy, and deployment baseline for `simsexam`.

## 1. Current Deployment Shape

Current assumptions:

- the service runs on a single Linux server
- the application listens only on `127.0.0.1:6080`
- public traffic is handled by a reverse proxy such as Nginx or Caddy

Out of scope for now:

- direct public listening by the application process
- multi-instance deployment
- Kubernetes assumptions

Current priority:

- maintainability
- rollback safety
- straightforward troubleshooting

## 2. Branch And Merge Policy

Required rules:

- never push directly to `main`
- all work must happen on branches
- only reviewed and tested changes should be proposed for merge
- `main` only accepts pull request merges

Recommended branch naming:

- `codex/<topic>`
- `feature/<topic>`
- `fix/<topic>`

## 3. Minimum PR Requirements

Every pull request must pass at least:

- `make test`
- `make build`

Recommended future additions:

- `gofmt` verification
- linting
- deeper integration coverage

## 4. GitHub Actions Policy

Current decision:

- use GitHub Actions for CI
- do not use GitHub Actions for production auto-deploy yet

Reasoning:

- the project is still evolving quickly
- automated validation is more urgent than automated deployment
- manual deployment remains easier to audit and roll back

Current GitHub Actions responsibilities:

- check out the repository
- set up Go
- run `make test`
- run `make build`
- publish CI artifacts and tagged release assets

Current GitHub Actions does not do:

- deploy to servers automatically
- replace running production processes
- execute production database operations automatically

## 5. Recommended Deployment Flow

Recommended stack:

- Linux
- `systemd`
- reverse proxy
- SQLite

Typical deployment steps:

1. obtain the target release artifact
2. install binaries on the server
3. run `simsexam-migrate` or `simsexam-bootstrapv1`
4. import additional content if needed
5. restart the `systemd` service
6. run a smoke test

For the standard single-host directory layout and systemd flow, see:

- [linux-deployment-layout.md](/Users/yu/repos/simsexam/docs/linux-deployment-layout.md:1)

## 6. Manual Pre-Deployment Checks

Even when CI passes, confirm at least:

- the home page loads
- an exam can be started
- answers can be submitted
- the admin import flow works
- single-question editing works

## 7. Runtime Conventions

The application should be managed by `systemd`.

Recommended conventions:

- an extracted self-contained release bundle under `/opt/simsexam/releases/<version>/`
- a stable `/opt/simsexam/current` symlink pointing to the active bundle
- database file under `/var/lib/simsexam/simsexam_v1.db`
- logs inspected through `journalctl`

## 8. Configuration Baseline

The deployment baseline should remain:

- listen on `127.0.0.1:6080`

Current runtime configuration entry points:

- `SIMSEXAM_ADDR`
- `SIMSEXAM_DB_PATH`
- `SIMSEXAM_IMPORT_SOURCE_TYPE`
- `SIMSEXAM_ADMIN_PASSWORD`
- `SIMSEXAM_ADMIN_SESSION_SECRET`

These are loaded centrally through `internal/config`. Runtime database settings are shared by `server`, `migrate`, `bootstrapv1`, and `importer`; admin access settings are used by `server`.

Operational recommendation:

- avoid spaces in `SIMSEXAM_ADMIN_PASSWORD`
- use a long random base64 or hex value without spaces for `SIMSEXAM_ADMIN_SESSION_SECRET`

## 9. Future Evolution Order

If deployment complexity grows later, use this order:

1. improve CI first
2. improve release packaging and versioned assets
3. evaluate deployment automation
4. only then consider container orchestration or Kubernetes

Not recommended at the current stage:

- complex CD pipelines
- Kubernetes-specific deployment work
- multi-environment orchestration

## 10. Current Summary

Current working policy:

1. Production runs on a single Linux host.
2. The application listens on `127.0.0.1:6080`.
3. All changes go through branches and pull requests.
4. Direct pushes to `main` are not allowed.
5. Pull requests must pass `make test` and `make build`.
6. GitHub Actions currently handles CI and release packaging, not production deployment.
7. Deployment remains manual, auditable, and rollback-friendly.
