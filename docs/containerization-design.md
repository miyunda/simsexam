# Containerization Design Note

This note outlines a practical containerization direction for `simsexam`.

The goal is not to redesign the system around orchestration. The goal is to make single-host deployment more consistent, self-contained, and easier to repeat.

## Why Consider Containerization

The current tarball-based release flow works, but it places several deployment responsibilities on the operator:

- extract the correct release bundle
- preserve the expected working directory layout
- keep binaries, templates, and static assets together
- install the correct `systemd` unit
- initialize or migrate the database in the right order

Those steps are manageable, but they increase the chance of drift between environments.

Containerization can reduce that drift by packaging the runtime assets and startup command into one deployable image.

## Scope

This design targets:

- one Linux host
- one running application instance
- SQLite persisted on a host volume
- reverse proxy handled either by the host or by a separate container later

This design does not assume:

- Kubernetes
- horizontal scaling
- immediate database replacement

## Recommended Outcome

Add Docker image distribution as a second release format, alongside the existing tarball release.

That means:

- keep the current tarball release flow
- add a container image release flow
- let operators choose the simpler path for their environment

This avoids turning a working release process into a migration project.

## What The Image Should Contain

The container image should be self-contained for application runtime.

Recommended contents:

- `simsexam`
- `simsexam-migrate`
- `simsexam-bootstrapv1`
- `templates/`
- `static/`

Optional additions:

- a default non-root runtime user
- an image label set with version and commit metadata

The image should not contain:

- a writable database file baked into the image
- environment-specific secrets

## Runtime Layout Inside The Container

The container should use a predictable application root, for example:

```text
/app/
  simsexam
  simsexam-migrate
  simsexam-bootstrapv1
  templates/
  static/
```

Recommended defaults:

- working directory: `/app`
- internal listen address: `0.0.0.0:6080`
- database path: `/data/simsexam_v1.db`

## SQLite Strategy

SQLite remains acceptable for the current stage of the project.

The important rule is:

- the database file must live on a mounted volume, not inside the ephemeral container layer

Recommended host-to-container mapping:

- host: `/var/lib/simsexam`
- container: `/data`

Then:

- `SIMSEXAM_DB_PATH=/data/simsexam_v1.db`

This preserves the database across image upgrades and container replacement.

## Initialization And Migration Strategy

There are two viable models.

### Model A: Explicit One-Shot Commands

Use dedicated one-shot containers for database preparation:

- run `simsexam-migrate`
- run `simsexam-bootstrapv1`
- then start the main service container

Advantages:

- explicit
- easier to reason about
- matches the current command separation
- cleaner long-term if deployment grows more structured

Tradeoffs:

- first deployment has an extra step
- upgrade instructions are slightly longer

### Model B: Application Startup Self-Preparation

Let the main application continue preparing the database during startup.

Advantages:

- shortest operator workflow
- familiar to the current server behavior

Tradeoffs:

- mixes application serving and environment preparation
- less clean if multi-instance deployment ever appears

### Recommendation

Prefer Model A for container deployment.

That keeps the operational model clearer and avoids hiding migration work inside the main service process.

## Reverse Proxy Strategy

Containerization does not remove the need for an ingress boundary.

Recommended short-term options:

### Option 1: Host-Level Reverse Proxy

Keep Caddy or Nginx on the host and proxy to the container port bound on loopback.

Example direction:

- container publishes `127.0.0.1:6080:6080`
- host Caddy proxies to `127.0.0.1:6080`

Advantages:

- minimal change from the current working setup
- TLS and LAN routing remain where they already work

### Option 2: Containerized Reverse Proxy

Run Caddy in Docker Compose alongside `simsexam`.

Advantages:

- more self-contained stack definition

Tradeoffs:

- more moving parts at once

### Recommendation

Start with host-level reverse proxy.

It reduces migration risk and keeps the first containerization step focused on the application itself.

## Single-Host Docker Compose Shape

The most practical first deployment target is Docker Compose.

Recommended services:

- `simsexam`

Optional later services:

- `caddy`
- backup helper

Environment variables for the app container:

- `SIMSEXAM_ADDR=0.0.0.0:6080`
- `SIMSEXAM_DB_PATH=/data/simsexam_v1.db`
- `SIMSEXAM_IMPORT_SOURCE_TYPE=manual`

Volume mapping:

- `/var/lib/simsexam:/data`

Port mapping:

- `127.0.0.1:6080:6080`

## Release Strategy

Container images should complement, not replace, the current release assets.

Recommended release outputs for a tagged version:

- tarball release bundle
- container image

Possible image naming:

- `ghcr.io/<owner>/simsexam:vX.Y.Z`
- `ghcr.io/<owner>/simsexam:latest` only if you later want a moving tag

Recommended metadata labels:

- version
- git commit
- source repository URL

## CI And Release Pipeline Impact

If containerization is implemented, the release pipeline will likely need:

- Docker build
- image tagging from Git tags
- image push to a registry such as GHCR

This should happen in the release workflow, not in the ordinary branch CI path.

Branch CI should remain focused on:

- tests
- build validation

## Operational Advantages

Containerization is worth doing because it improves:

- release consistency
- environment reproducibility
- operator ergonomics
- separation between host data and application runtime

It also reduces the chance of repeating earlier deployment issues involving:

- missing templates
- missing static assets
- mismatched working directories

## Operational Caveats

Containerization does not eliminate every deployment concern.

It still requires:

- volume management
- database backup strategy
- migration discipline
- reverse proxy configuration
- release tagging discipline

It also does not change the architectural limits of SQLite for future horizontal scaling.

## Recommended Rollout

### Phase 1

- add a `Dockerfile`
- define the application runtime layout in the image
- keep tarball releases unchanged

### Phase 2

- add a `docker-compose.yml` for single-host deployment
- document one-shot migration and bootstrap commands

### Phase 3

- publish versioned images from the release workflow
- optionally publish to GHCR

## Recommendation

`simsexam` should adopt containerization as an additional release and deployment format.

The first implementation should stay disciplined:

- one image
- one host
- SQLite on a mounted volume
- explicit migration and bootstrap commands
- host-level reverse proxy retained at first

That approach delivers most of the practical deployment benefits without prematurely pushing the project into orchestration complexity.
