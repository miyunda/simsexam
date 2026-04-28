# simsexam Linux Deployment Layout

This document defines the standard directory layout, configuration locations, logging approach, and deployment steps for `simsexam` on a single Linux server.

Current target:

- single-host deployment
- `systemd` process supervision
- application listens only on `127.0.0.1:6080`
- SQLite for local persistent storage
- reverse proxy handles public traffic

Assumptions:

- You download `simsexam-<version>-<os>-<arch>.tar.gz` from GitHub Releases
- The archive expands into a self-contained application bundle directory
- The bundle contains:
  - `simsexam`
  - `simsexam-migrate`
  - `simsexam-bootstrapv1`
  - `simsexam.service.template`
  - `simsexam.env.example`
  - `templates/`
  - `static/`
- The default demo seed is embedded in the binaries and does not require `docs/examples/se-demo.md` on the server

## 0. Shell Variables

Use shell variables so the same commands work across releases:

```bash
VERSION="vX.Y.Z"
OS="linux"
ARCH="amd64"
ARCHIVE="simsexam-${VERSION}-${OS}-${ARCH}.tar.gz"
BUNDLE_DIR="simsexam-${VERSION}-${OS}-${ARCH}"
RELEASE_DIR="/opt/simsexam/releases/${VERSION}"
CURRENT_LINK="/opt/simsexam/current"
DB_PATH="/var/lib/simsexam/simsexam_v1.db"
```

When upgrading, change `VERSION` to the target release and reuse the same pattern. The current official release build target is `linux-amd64`.

## 1. Standard Directory Layout

Use this layout consistently:

```text
/opt/simsexam/
  current -> /opt/simsexam/releases/<version>/simsexam-<version>-<os>-<arch>
  releases/
    <version>/
      simsexam-<version>-<os>-<arch>/
        simsexam
        simsexam-migrate
        simsexam-bootstrapv1
        simsexam.service.template
        simsexam.env.example
        templates/
        static/

/etc/simsexam/
  simsexam.env

/var/lib/simsexam/
  simsexam_v1.db

/var/log/simsexam/
```

Responsibility of each path:

- `/opt/simsexam/releases/<version>/`
  - immutable extracted release bundle for a specific version
- `/opt/simsexam/current`
  - stable symlink to the active release bundle
- `/etc/simsexam/simsexam.env`
  - runtime configuration
- `/var/lib/simsexam/simsexam_v1.db`
  - SQLite database file
- `/var/log/simsexam/`
  - reserved log directory; `journald` remains the primary log sink

## 2. Why This Layout

This layout keeps deployment predictable:

- binaries and runtime assets stay together
- relative paths for `templates/` and `static/` work reliably
- upgrades only need a new release directory and a symlink switch
- configuration and data stay outside the application bundle
- rollback is just a symlink change plus service restart

## 3. Standard Environment File

Create `/etc/simsexam/simsexam.env` with:

```bash
SIMSEXAM_ADDR=127.0.0.1:6080
SIMSEXAM_DB_PATH=/var/lib/simsexam/simsexam_v1.db
SIMSEXAM_IMPORT_SOURCE_TYPE=manual
SIMSEXAM_ADMIN_PASSWORD=change-this-before-production
SIMSEXAM_ADMIN_SESSION_SECRET=change-this-to-a-long-random-secret
```

You can start from the bundled `simsexam.env.example`.

Field notes:

- `SIMSEXAM_ADDR`
  - keep the application bound to loopback only
- `SIMSEXAM_DB_PATH`
  - points to the canonical database file
- `SIMSEXAM_IMPORT_SOURCE_TYPE`
  - retained as a configurable runtime value
- `SIMSEXAM_ADMIN_PASSWORD`
  - required to access `/admin/*`
  - use a random 24-32 character value without spaces
- `SIMSEXAM_ADMIN_SESSION_SECRET`
  - used to sign the admin session cookie; must be long and random in production
  - use a base64 or hex secret without spaces

## 4. systemd Unit Location

Install the service unit at:

```text
/etc/systemd/system/simsexam.service
```

The release bundle includes a ready-to-copy template:

- `simsexam.service.template`

## 5. First Deployment Steps

### 5.1 Create User and Directories

```bash
sudo useradd --system --home /opt/simsexam --shell /usr/sbin/nologin simsexam || true

sudo mkdir -p "$RELEASE_DIR"
sudo mkdir -p /etc/simsexam
sudo mkdir -p /var/lib/simsexam
sudo mkdir -p /var/log/simsexam

sudo chown -R simsexam:simsexam /opt/simsexam /var/lib/simsexam /var/log/simsexam
sudo chown root:root /etc/simsexam
```

### 5.2 Install the Release Bundle

After uploading the release archive to the server:

```bash
tar -xzf "$ARCHIVE"
sudo mv "$BUNDLE_DIR" "$RELEASE_DIR/"
sudo ln -sfn "${RELEASE_DIR}/${BUNDLE_DIR}" "$CURRENT_LINK"
sudo chown -R simsexam:simsexam "$RELEASE_DIR"
```

### 5.3 Write the Environment File

Start from the bundled example file:

```bash
sudo cp /opt/simsexam/current/simsexam.env.example /etc/simsexam/simsexam.env
```

Edit `/etc/simsexam/simsexam.env` if needed. The default content is:

```bash
SIMSEXAM_ADDR=127.0.0.1:6080
SIMSEXAM_DB_PATH=/var/lib/simsexam/simsexam_v1.db
SIMSEXAM_IMPORT_SOURCE_TYPE=manual
SIMSEXAM_ADMIN_PASSWORD=change-this-before-production
SIMSEXAM_ADMIN_SESSION_SECRET=change-this-to-a-long-random-secret
```

### 5.4 Install the systemd Unit

Copy the template from the extracted release bundle:

```bash
sudo cp /opt/simsexam/current/simsexam.service.template /etc/systemd/system/simsexam.service
sudo systemctl daemon-reload
```

### 5.5 Initialize the Database

For first deployment, explicitly initialize the database before starting the service:

```bash
sudo -u simsexam /opt/simsexam/current/simsexam-migrate -dsn "$DB_PATH"
sudo -u simsexam /opt/simsexam/current/simsexam-bootstrapv1 -dsn "$DB_PATH"
```

If you want the minimum first-run path, `simsexam-bootstrapv1` is usually enough because it prepares the v1 schema before seeding.

### 5.6 Start the Service

```bash
sudo systemctl enable --now simsexam
sudo systemctl status simsexam
curl http://127.0.0.1:6080/
```

## 6. Database File Location

The database path is controlled by `SIMSEXAM_DB_PATH`.

Under this deployment standard, the file lives at:

```text
/var/lib/simsexam/simsexam_v1.db
```

If the file does not exist, prefer explicit initialization with the bundled `simsexam-migrate` and `simsexam-bootstrapv1` binaries before starting the service.

## 7. Logging

Use `journald` as the primary log entry point:

```bash
sudo systemctl status simsexam
sudo journalctl -u simsexam -n 100 --no-pager
sudo journalctl -u simsexam -f
```

`/var/log/simsexam/` is reserved for future use and should not be treated as the primary runtime log location today.

## 8. Upgrade Steps

Example upgrade flow:

```bash
VERSION="vX.Y.Z"
OS="linux"
ARCH="amd64"
ARCHIVE="simsexam-${VERSION}-${OS}-${ARCH}.tar.gz"
BUNDLE_DIR="simsexam-${VERSION}-${OS}-${ARCH}"
RELEASE_DIR="/opt/simsexam/releases/${VERSION}"
CURRENT_LINK="/opt/simsexam/current"
DB_PATH="/var/lib/simsexam/simsexam_v1.db"

tar -xzf "$ARCHIVE"
sudo mkdir -p "$RELEASE_DIR"
sudo mv "$BUNDLE_DIR" "$RELEASE_DIR/"
sudo chown -R simsexam:simsexam "$RELEASE_DIR"

sudo ln -sfn "${RELEASE_DIR}/${BUNDLE_DIR}" "$CURRENT_LINK"
sudo -u simsexam /opt/simsexam/current/simsexam-migrate -dsn "$DB_PATH"
sudo systemctl restart simsexam
```

Recommended checks immediately after upgrade:

```bash
sudo systemctl status simsexam
curl http://127.0.0.1:6080/
```

## 9. Rollback Steps

If the new release is faulty, switch `current` back to the previous release bundle:

```bash
PREVIOUS_VERSION="vPREVIOUS"
OS="linux"
ARCH="amd64"
PREVIOUS_BUNDLE_DIR="simsexam-${PREVIOUS_VERSION}-${OS}-${ARCH}"
PREVIOUS_RELEASE_DIR="/opt/simsexam/releases/${PREVIOUS_VERSION}"

sudo ln -sfn "${PREVIOUS_RELEASE_DIR}/${PREVIOUS_BUNDLE_DIR}" /opt/simsexam/current
sudo systemctl restart simsexam
```

## 10. Reverse Proxy Boundary

`simsexam` itself should only listen on:

```text
127.0.0.1:6080
```

Public traffic should be terminated and forwarded by Nginx or Caddy.

That means:

- `simsexam` is not directly exposed on a public interface
- TLS termination belongs in the reverse proxy layer
- if the system later moves to multi-instance deployment, the ingress design should be revisited then
