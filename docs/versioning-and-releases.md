# Versioning And Releases

## Policy

`simsexam` uses Semantic Versioning tags as the source of truth for releases.

Examples:

- `v0.1.0`
- `v0.1.1`
- `v0.2.0`
- `v1.0.0`

Meaning:

- `MAJOR`: incompatible external change
- `MINOR`: backward-compatible feature release
- `PATCH`: backward-compatible bug fix

## Branching Rule

- Never push directly to `main`
- Merge to `main` only through reviewed pull requests
- Create a release only from a known-good commit

## Local Build Metadata

The server binary embeds:

- version
- commit
- build time

You can inspect the current metadata with:

```bash
make version
./bin/simsexam --version
```

## Release Process

1. Merge the tested change set into `main`
2. Create and push a release tag, for example:

```bash
VERSION="vX.Y.Z"
OS="linux"
ARCH="amd64"
git tag "$VERSION"
git push origin "$VERSION"
```

3. GitHub Actions `Release` workflow will:
   - verify formatting
   - run tests
   - build the current official release target, `linux-amd64`
   - package `simsexam-${VERSION}-${OS}-${ARCH}.tar.gz`
   - generate `simsexam-${VERSION}-SHA256SUMS.txt`
   - publish a GitHub Release with downloadable assets

Current release package contents:

- `simsexam`
- `simsexam-migrate`
- `simsexam-bootstrapv1`
- `simsexam.service.template`
- `simsexam.env.example`
- `templates/`
- `static/`

Current official release target:

- `linux-amd64`

The release naming scheme already reserves `OS` and `ARCH` in package names so future targets such as `linux-arm64` can be added without changing deployment conventions.

## CI Artifacts vs Releases

- `CI` artifacts are for branch and pull-request validation
- `Release` assets are for formal versioned distribution
- Only Git tags matching the release format, such as `vX.Y.Z`, should produce official downloadable release binaries

## Tag Discipline

Do not replace or silently mutate an existing published release tag.

If release contents change in a user-visible or deployment-relevant way, publish a new version tag.

Examples:

- `v0.1.0`: first formal release
- `v0.1.1`: same release line, but improved packaging or deployment tooling

## Testing Flow Reference

The release process assumes the candidate commit has already passed internal and
staging validation before it is tagged.

See:

- [pr-testing-and-release-flow.md](/Users/yu/repos/simsexam/docs/pr-testing-and-release-flow.md:1)
