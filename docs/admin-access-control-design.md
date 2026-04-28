# Admin Access Control Design Note

This note describes a practical access-control plan for the `simsexam` admin area.

The immediate problem is simple:

- the admin routes are currently accessible to unauthenticated visitors

That is acceptable for a prototype, but it should not remain true as the product grows.

## Goal

Protect all admin routes with a minimal but credible authentication boundary now, without blocking future migration to a full user and role system.

## Current State

The application already exposes admin pages such as:

- `/admin/subjects`
- `/admin/import`
- `/admin/subjects/{id}/questions`
- `/admin/questions/{id}/edit`

These routes currently have no access-control middleware.

## Recommendation

Implement admin protection in two phases.

### Phase A

Add a lightweight admin gate that is independent from the future end-user login system.

### Phase B

When full user authentication is added, migrate admin access to the normal user identity and role model.

This separates:

- immediate route protection
- long-term account architecture

## Phase A: Lightweight Admin Gate

### User Experience

Introduce:

- `/admin/login`
- `/admin/logout`

Behavior:

- unauthenticated requests to `/admin/*` redirect to `/admin/login`
- a successful admin login creates an authenticated admin session
- `/admin/logout` clears that session

This protects the admin area without requiring the full user login project first.

### Credential Model

Use a deployment-level shared admin secret, configured through environment variables.

Recommended configuration:

- `SIMSEXAM_ADMIN_PASSWORD`
- `SIMSEXAM_ADMIN_SESSION_SECRET`

Meaning:

- `SIMSEXAM_ADMIN_PASSWORD`: the password required to enter the admin area
- `SIMSEXAM_ADMIN_SESSION_SECRET`: the secret used to sign the admin session cookie

This is intentionally simple and should be treated as an operator-level admin gate, not as a permanent multi-admin identity system.

### Session Model

Use a signed server-trusted cookie.

Recommended properties:

- `HttpOnly`
- `Secure` when served behind HTTPS
- `SameSite=Lax`
- bounded lifetime, for example 8 or 24 hours

The cookie should not store the password itself.

Instead, it should store a signed session payload that indicates:

- admin authenticated
- issued at
- expires at

The signature should be verified on every admin request.

### Route Protection Model

Use a single middleware for all `/admin/*` routes except:

- `/admin/login`

Recommended behavior:

- if admin session is valid, continue
- if not valid, redirect to `/admin/login`

This should be implemented at the router level, not repeated inside each handler.

### Failure Behavior

For the first version:

- incorrect password returns the login form with a generic error
- no information should reveal whether the admin password is configured incorrectly

### Operational Requirement

If `SIMSEXAM_ADMIN_PASSWORD` is missing, the admin gate should fail closed.

Recommended behavior:

- deny admin login
- deny access to admin routes
- log a clear startup or access warning for operators

The admin area should never silently remain open because a password was not configured.

## Why Phase A Is Worth Doing

This approach has several advantages:

- protects the admin area quickly
- does not depend on Google, Apple, or full end-user auth
- does not require building a database-backed admin account model yet
- can be replaced later without changing all admin handlers

It is the right fit for the current project stage, where:

- administrator count is very small
- deployment is self-hosted
- the broader user login strategy is still evolving

## Why Not Use Only Reverse Proxy Auth

Reverse-proxy protection such as Caddy or Nginx basic auth can be useful as an outer layer, but it should not be the only application-level protection plan.

Reasons:

- it lives outside the application logic
- it does not help with future role-based migration
- it does not provide application-aware login and logout behavior
- it is harder to reuse if deployment topology changes

A proxy-level restriction may still be useful as defense in depth, but it should not replace an application-level admin gate.

## Phase B: Migrate To Role-Based Admin Access

Later, when normal user login exists, admin access should move to the main user model.

The schema already points in that direction with:

- `users`
- `user_identities`
- `users.role`

The target model should become:

- unauthenticated user -> redirect to standard login
- authenticated non-admin user -> return `403 Forbidden`
- authenticated admin user -> allow access

At that point:

- `SIMSEXAM_ADMIN_PASSWORD` can be deprecated
- the admin session can become part of the normal authenticated user session

## Migration Strategy

Phase A should be implemented in a way that makes Phase B straightforward.

Recommended design choices:

- isolate session parsing into middleware helpers
- avoid hardcoding password checks into admin handlers
- keep route protection centralized
- structure the middleware so the current credential source can later change from:
  - shared password
  - to authenticated user role

## Recommended Middleware Responsibilities

The admin middleware should:

- read the admin session cookie
- verify the signature
- validate expiration
- reject invalid or expired sessions
- redirect to `/admin/login` when needed

It should not:

- perform database lookups in Phase A
- know about ordinary user login yet

## UI Considerations

The admin login page should be minimal.

Recommended fields:

- password
- submit button

Recommended actions:

- successful login redirects to `/admin/subjects`
- logout redirects to `/admin/login`

The normal public navigation should not expose unnecessary admin-only links once access control exists, though showing a link to the login page itself may still be acceptable.

## Security Considerations

Minimum expectations for Phase A:

- signed cookie, not plaintext trust
- finite session lifetime
- admin secret configured out of band
- no password stored in client-visible form
- fail closed if password is missing

Later improvements that may be worth adding:

- rate limiting on `/admin/login`
- audit log entries for admin login and logout
- CSRF protection for admin POST routes

## Testing Considerations

Minimum tests for a future implementation:

- unauthenticated request to `/admin/subjects` redirects to `/admin/login`
- valid admin login grants access
- invalid password keeps the user on the login page
- logout clears access
- missing `SIMSEXAM_ADMIN_PASSWORD` denies admin access

## Recommended Rollout

### Phase A

- add admin login page
- add admin logout endpoint
- add signed-cookie session helper
- protect all admin routes with middleware

### Phase B

- add standard end-user login
- connect identities and roles
- migrate admin access to user role checks
- retire the shared admin password model

## Recommendation

`simsexam` should add a lightweight application-level admin gate now.

The first implementation should be:

- password-based
- cookie-backed
- middleware-enforced
- fail-closed

That gives the project a credible security boundary immediately while preserving a clean migration path to full role-based administration later.
