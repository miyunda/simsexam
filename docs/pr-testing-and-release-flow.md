# PR Testing And Release Flow

This document defines the intended path from feature work to production release.

The key policy is:

- `staging` deploys a `commit artifact`
- `prod` deploys a tagged `release artifact`
- a release tag is created only after the same commit has passed internal and public staging validation

## 1. Goals

This flow exists to keep three concerns separate:

- pull-request validation
- pre-release environment testing
- formal versioned production release

That separation avoids using Git tags as disposable test markers.

## 2. Artifact Types

### Commit Artifact

A `commit artifact` is a build produced from one specific commit SHA before a formal release tag exists.

It is used for:

- PR verification
- internal test deployment
- public staging deployment

It is not used as a formal production release.

### Release Artifact

A `release artifact` is a build produced from a formal Git tag such as `vX.Y.Z`.

It is used for:

- GitHub Releases
- production deployment
- versioned rollback targets

## 3. Required Commit Identity Rule

The workflow depends on one strict assumption:

- the commit tested in PR and staging is the same commit that ends up in `main`
- the formal release tag is created from that same commit

If this rule stops being true later, the workflow must be revised before relying on it.

## 4. Environment Roles

### Internal Test Machine

The internal test machine is the first deployment target for a candidate commit.

Purpose:

- verify that the candidate artifact starts and behaves correctly in a controlled environment
- catch obvious operator and migration issues before public staging

Source of deployed artifact:

- the PR commit artifact

### Public Staging

The public staging environment validates the same candidate artifact under more realistic access conditions.

Purpose:

- validate the deployable artifact in a publicly reachable environment
- test real proxy, network, and operator workflows before release

Source of deployed artifact:

- the same PR commit artifact already used in internal testing

### Production

Production only receives formal release artifacts.

Purpose:

- run versioned, auditable, rollback-friendly builds

Source of deployed artifact:

- the GitHub Release asset produced from the formal tag

## 5. End-To-End Flow

The intended flow is:

1. Create a feature branch.
2. Open a Draft PR.
3. Run automated checks and Hermes review on the PR.
4. Build a commit artifact for the PR head commit.
5. Deploy that commit artifact to the internal test machine.
6. Deploy the same commit artifact to public staging.
7. After internal and staging validation pass, merge the PR to `main`.
8. Create a formal tag on that same commit in `main`.
9. Build and publish the formal GitHub Release from that tag.
10. Deploy the release artifact to production.

## 6. Required Gates

The following gates should hold:

- a PR must stay Draft until the change is ready for meaningful review
- CI must pass before the artifact is treated as deployable
- internal testing must pass before public staging deployment is accepted
- public staging must pass before merge and tag creation
- production must deploy only formal release artifacts

## 7. Tag Discipline

Tags are not for exploratory testing.

Tags should mean:

- this commit has already passed the required validation flow
- this version is suitable for formal release tracking
- this version is eligible for production deployment

As a result:

- do not create release tags for pre-staging experiments
- do not deploy arbitrary commit artifacts directly to production
- do not treat GitHub Releases as branch validation artifacts

## 8. Hermes Integration Status

Hermes review belongs in the PR validation phase.

Current limitation:

- Hermes is not yet exposed in a way that supports the intended webhook-driven integration path

Current policy until that is solved:

- keep Hermes review as a planned or manually triggered validation step
- do not block the rest of the documented release flow on public Hermes connectivity work

## 9. Operational Benefits

This workflow provides:

- cleaner release history
- fewer test-only tags
- clearer rollback targets
- better separation between SHA-based testing and tag-based production release
- more credible auditability for what actually reached production

## 10. Release Readiness

A PR passing CI and staging does not automatically mean it should become a
formal release.

A formal tag should be created only when the candidate change set forms a
coherent, supportable release slice.

Practical release readiness criteria:

- the version solves a clearly defined problem or delivers one coherent user-facing capability
- the main user or operator path is closed rather than partially implemented
- configuration, migration, smoke testing, and rollback expectations are clear
- the artifact and documentation semantics are consistent across PR testing, staging, and production
- the team is willing to keep that tag in the long-term release history as a real version

Current guidance for `simsexam`:

- infrastructure-only progress can stay in Draft PRs and staging without forcing a release tag
- a new tag is more appropriate once a user-visible workflow is meaningfully closed

Example for the current Phase 3 direction:

- anonymous sessions alone are not a strong release boundary
- login plus history claim plus learner review or mistake workflows would be a stronger release candidate

## 11. Relationship To Other Docs

This document defines environment flow and artifact semantics.

Related references:

- [versioning-and-releases.md](/Users/yu/repos/simsexam/docs/versioning-and-releases.md:1)
- [testing-and-deployment.md](/Users/yu/repos/simsexam/docs/testing-and-deployment.md:1)
- [linux-deployment-layout.md](/Users/yu/repos/simsexam/docs/linux-deployment-layout.md:1)
