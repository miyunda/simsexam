# simsexam Roadmap

This roadmap organizes the next stages of development for `simsexam` after the test deployment reached a stable baseline.

The project now has:

- a working Go monolith
- a v1 database schema
- Markdown import
- admin subject and question management
- edit history foundations
- self-contained release bundles
- basic admin access protection
- stable single-host deployment
- validated commit-artifact deployment to the `Staging` environment
- learner registration and login
- anonymous learning sessions that can be claimed after registration or login
- first-version question quality feedback workflows

The next step is to turn the newly introduced identity foundation into a
visible learner workflow.

## Guiding Priorities

The roadmap is ordered around these priorities:

1. keep the deployed system stable
2. improve question-bank quality workflows
3. improve user learning workflows
4. improve long-term release and operations maturity

## Phase 1: Stabilize The Current Operational Baseline

### Goal

Turn the current single-host deployment and admin workflow into a repeatable, low-friction baseline.

### Planned Work

- completed: validate repeatable staging deployment from CI commit artifacts
- completed: document PR, staging, and release-promotion flow
- completed: bind staging deployment secrets to the `Staging` environment
- remaining: verify production-oriented admin protection behavior in real deployment
- remaining: review admin login and rate-limit logs after public exposure
- remaining: tighten deployment and smoke-test documentation where operator friction still exists
- completed: refine admin status workflows around:
  - archived subjects
  - disabled questions
- completed: improve status visibility in admin pages
- completed: clarify revision summaries for admin actions

### Exit Criteria

- completed: test deployment remains stable under real operator usage
- completed: release, deploy, and rollback instructions are reliable enough for the current staging flow
- remaining: admin access control and login rate limiting behave predictably after public production exposure

## Phase 2: Strengthen Question-Bank Quality Workflows

### Goal

Improve the system's ability to manage, review, and correct question-bank content over time.

### Planned Work

- completed: expose option-shuffling controls in admin UI:
  - subject-level `shuffle_options_default`
  - question-level `allow_option_shuffle`
- completed: add question revision and history viewing
- completed: implement learner question feedback first version:
  - learner-side structured report flow
  - admin feedback list
  - resolve and dismiss actions
- remaining: improve importer validation quality and warning clarity
- remaining: continue refining admin content-editing ergonomics

### Why This Phase Matters

The question bank is a core product asset. This phase focuses on improving:

- content quality
- reviewability
- correction speed
- auditability

### Exit Criteria

- completed: admins can configure option shuffling intentionally
- completed: admins can inspect question history without reading raw database tables
- completed: learners have a structured way to report flawed questions

## Phase 3: Add User Identity And Learning Workflows

### Goal

Move from anonymous practice sessions toward durable user learning records.

### Planned Work

- completed: implement first-pass email/password user authentication
- completed: add durable anonymous learning sessions
- completed: claim current-browser anonymous exams and feedback after registration or login
- completed in current iteration: implement learner mistake tracking and a first wrong-answer notebook flow
- completed in current iteration: build user-facing review pages for weak questions or repeated mistakes
- later: connect admin access to `users.role = admin`
- later: retire the shared admin password model after role-based admin auth is ready
- later: support third-party identity integration planning or first rollout
- later: prepare subject entitlement structure for future paid content

Scope note:

- Phase 3 needs minimum admin-facing user operations for role assignment and entitlement handling
- Phase 3 does not require a full user-management console

See [user-identity-learning-design.md](user-identity-learning-design.md) for the
current Phase 3 design brainstorm around anonymous sessions, account linking,
email verification, wrong-answer notebooks, and future entitlements.

### Why This Phase Matters

The product becomes more valuable when it helps users improve over time, not only complete one exam session.

### Exit Criteria

- user sessions are linked to persistent identities
- admin access is role-based instead of shared-secret-based
- learners can review their own mistakes across sessions

## Phase 4: Expand Release And Deployment Options

### Goal

Add more portable release options without disrupting the current stable tarball deployment model.

### Planned Work

- add Docker image build support
- add a single-host Docker Compose deployment path
- keep tarball and container releases in parallel
- document data persistence and backup expectations for container deployment
- decide whether tagged releases should also publish container images

### Why This Phase Matters

The current deployment works, but containerization can reduce operator setup drift and improve release portability.

### Exit Criteria

- container deployment exists as a supported option
- tarball deployment remains supported and documented
- operators can choose the release form that best fits their environment

## Phase 5: Operational Hardening

### Goal

Improve observability, safety, and maintainability once the main product workflows are more complete.

### Planned Work

- add clearer audit logging for admin actions
- improve backup and restore documentation
- define database migration operational expectations more explicitly
- add stronger smoke-test and release checklists
- consider structured logs or log enrichment for security-sensitive paths
- evaluate whether more proxy-side protection is needed for admin paths

### Exit Criteria

- releases are easier to validate before rollout
- operational recovery procedures are documented
- sensitive workflows have adequate observability

## Immediate Next Iteration Recommendation

The immediate Phase 2 sequence has been completed:

1. expose option-shuffling controls in admin UI
2. add question revision/history viewing
3. implement question feedback first version

The current short iteration implements:

1. implement `user_question_stats` updates during answer submission
2. add `/me/mistakes` as the first wrong-answer notebook page
3. add a question review page linked from the notebook
4. rebuild or refresh stats when an anonymous session is claimed by a user

This sequence stays close to the product's core value:

- question quality
- reviewability
- learner trust
- visible learner progress

## What Is Deliberately Not Prioritized Yet

The roadmap does not currently prioritize:

- Kubernetes-specific deployment work
- multi-instance scaling
- complex payment implementation
- large-scale notification systems
- generalized customer-support tooling

Those can be revisited later if the product and user base justify them.

## Summary

The current roadmap direction is:

- keep the validated deployment baseline stable
- continue polishing question-bank quality workflows as issues appear
- invest next in real learner continuity through mistake tracking
- then broaden release and deployment options

This keeps `simsexam` moving toward a professional product without prematurely optimizing for scale it does not need yet.
