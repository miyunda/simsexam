# User Identity And Learning Workflow Design Note

This note captures early Phase 3 product and technical direction. It is not an
implementation plan yet. The goal is to keep the user-identity ideas coherent
before the project commits to database changes, authentication providers, or
paid-content behavior.

## Product Goal

Move `simsexam` from anonymous one-off practice toward durable learning records.

The first useful outcome is:

- a learner can practice before registering
- the system can preserve that local anonymous history
- the learner can later create or link an account
- exams, mistakes, feedback, and future entitlements can attach to one durable
  user identity

## Guiding Principles

- Do not force registration before first practice.
- Do not let email delivery infrastructure block the first identity iteration.
- Support one user account with multiple login identities.
- Treat account merging as a sensitive operation that needs explicit proof and
  audit history.
- Keep payment integration out of the first identity pass; prepare entitlement
  structure first.

## Anonymous Learning Sessions

Anonymous practice should have a durable browser-local identity so useful
history is not thrown away before registration.

Recommended model:

```text
anonymous_sessions
- id
- token_hash
- created_at
- last_seen_at
- claimed_user_id NULL
- claimed_at NULL
```

Cookie behavior:

- generate a high-entropy random token on first anonymous visit
- store only the token hash in the database
- set the browser cookie as `HttpOnly`, `Secure` behind HTTPS, and
  `SameSite=Lax`
- do not use browser fingerprinting

When an anonymous learner starts an exam:

```text
exams.user_id = NULL
exams.anonymous_session_id = <current anonymous session>
```

When the learner registers or logs in from the same browser, the system can
claim the anonymous session:

- set `anonymous_sessions.claimed_user_id`
- set `anonymous_sessions.claimed_at`
- migrate or associate eligible anonymous exams and feedback with the user
- rebuild or refresh learner question stats from the claimed history

The claim should only apply to the current browser cookie. Cross-device history
requires an authenticated user account.

## Email And Password Authentication

The first authentication pass can support email and password without requiring
email verification.

Email verification is still worth designing for:

```text
users.email_verified_at NULL
```

Initial behavior:

- users can register and log in before email verification exists
- verification can be required later for sensitive flows such as password reset,
  email changes, paid content, or admin elevation
- the mailer implementation can be a no-op or log-only adapter in test
  deployments

Future mailer abstraction:

```go
type Mailer interface {
    SendVerificationEmail(to, link string) error
    SendPasswordResetEmail(to, link string) error
}
```

Real email delivery will require a provider such as Resend, Postmark, Mailgun,
SendGrid, or AWS SES, plus SPF, DKIM, and DMARC DNS configuration.

## Third-Party Identity Linking

The existing `users` and `user_identities` model is a good foundation for one
user account with multiple login methods.

Example:

```text
users
- id = 123
- email = learner@example.com

user_identities
- user_id = 123, provider = email, provider_user_id = learner@example.com
- user_id = 123, provider = google, provider_user_id = google-sub-abc
- user_id = 123, provider = github, provider_user_id = github-id-xyz
```

Recommended first behavior:

- a logged-in user can link a new third-party identity from account settings
- the user must prove control of the new identity through that provider's login
  flow
- if the identity is unclaimed, attach it to the current user
- if the identity already belongs to another user, do not auto-merge accounts in
  the first version

## Account Merging

Account merging is a reasonable future need, especially when learners use
different devices or providers before realizing they have split history.

It should not be automatic.

A safe merge requires the learner to prove control of both accounts. A future
merge flow should show what will move:

- exams
- question feedback
- learner question stats
- subject entitlements
- linked identities

Recommended future audit model:

```text
account_merge_events
- id
- source_user_id
- target_user_id
- initiated_by_user_id
- merged_at
- reason
- snapshot_json
```

The source user should be marked as merged or disabled instead of silently
deleted.

## Learner Records

Once a request has an authenticated user or an anonymous session, new learning
data should preserve that owner.

Candidate associations:

```text
exams.user_id
exams.anonymous_session_id
question_feedback.user_id
question_feedback.anonymous_session_id
```

The first durable learner value should be a wrong-answer notebook rather than a
large personal dashboard.

When a logged-in learner submits an answer:

- increment `user_question_stats.wrong_count` when incorrect
- increment `user_question_stats.correct_count` when correct
- update `last_answered_at`
- update `last_wrong_at` when incorrect
- maintain a simple `mastery_status`

Initial mastery behavior can stay conservative:

```text
new       no meaningful history yet
weak      the learner has answered this question incorrectly
mastered  the learner has later answered it correctly enough times
```

## Wrong-Answer Notebook

First version pages:

- `/me/mistakes`
- optional subject filter
- row for question key, stem summary, wrong count, last wrong time, and mastery
  status
- link to a review page with stem, options, correct answer, explanation, and
  learner stats

This should be the first user-facing payoff after login.

## Admin Access Migration

Admin access should eventually move from the shared deployment password to the
normal user model:

```text
users.role = admin
users.status = active
```

Recommended transition:

1. keep the shared admin password as a fallback
2. add normal user login
3. allow admin users to access `/admin/*`
4. verify role-based admin access in deployment
5. retire or disable the shared admin password model

## First Admin Bootstrap

The first `users.role = admin` account should not appear through automatic
application logic.

Recommended bootstrap rule:

- the user registers through the normal login flow
- a trusted operator promotes that account through a direct server-side action
- the simplest acceptable first implementation is a direct database update

Example operator action:

```sql
UPDATE users
SET role = 'admin'
WHERE email = 'admin@example.com';
```

This keeps the bootstrap path explicit and auditable while role-based admin
access is still being introduced.

Not recommended:

- automatically promoting the first registered user
- promoting users based on email pattern alone
- hidden bootstrap routes or one-time public URLs
- any self-service elevation path before the admin model is proven

## Minimal Admin User Operations

The near-term requirement is not a full user-management console.

What is needed sooner is a narrow set of administrator operations directly tied
to authorization and supportable rollout:

- identify which users have `role = admin`
- grant or remove admin access in a controlled way
- inspect enough user identity information to apply role or entitlement changes safely
- later, grant or revoke subject entitlements manually

This is intentionally smaller than a general-purpose admin user management
system.

Not a first-pass requirement:

- a full user list and search console
- broad profile editing
- account merge tooling
- general support operations unrelated to auth, entitlement, or rollout safety
- password reset back-office tools before the email and account recovery model is clearer

## Entitlements

Paid content should not be implemented in the first identity pass, but the data
model should prepare for it.

The first entitlement iteration can support manual grants:

- `subjects.access_level = free | paid | private`
- check entitlement before starting non-free exams
- let admins grant or revoke subject access manually

Payment integration can come later after the authorization model is proven.

## Suggested Implementation Sequence

1. `d/anonymous-sessions`
   - durable anonymous cookie
   - anonymous session table
   - attach anonymous exams to the session

2. `d/user-auth-basic`
   - email/password registration and login
   - session cookies
   - email verification fields but no required email delivery

3. `d/claim-anonymous-history`
   - claim current anonymous session after registration or login
   - associate prior exams and feedback with the user

4. `d/admin-role-auth`
   - allow `users.role = admin` into admin routes
   - keep shared admin password as fallback during transition
   - add only the minimum admin-facing user operations needed for role assignment

5. `d/mistake-notebook`
   - populate `user_question_stats`
   - first wrong-answer notebook pages

6. `d/account-linking`
   - allow logged-in users to attach additional identity providers
   - explicitly reject automatic merge when an identity belongs to another user

7. `d/subject-entitlements`
   - enforce free/paid/private subject access
   - add admin manual entitlement grants

## Open Questions

- Should anonymous exams become visible immediately after login, or should the
  user explicitly confirm claiming local history?
- How long should anonymous sessions live?
- Should password reset wait for real email delivery, or should the first admin
  tooling include manual password reset?
- Which third-party identity provider is most valuable for the first rollout?
- Should account merging be admin-assisted only at first?
