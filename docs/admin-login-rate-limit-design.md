# Admin Login Rate Limiting Design Note

This note describes a practical rate-limiting approach for the `simsexam` admin login flow.

The design is intentionally compatible with different reverse-proxy stacks, including:

- Cloudflare -> Caddy -> simsexam
- Cloudflare -> Nginx -> simsexam
- direct reverse proxy on the same host without Cloudflare

The design should not depend on one specific proxy product.

## Goal

Reduce the effectiveness of brute-force attempts against `/admin/login` while keeping the implementation simple enough for the current single-host deployment model.

## Current Context

The admin area is moving to a lightweight password-based gate.

That creates a new requirement:

- repeated failed password attempts must be slowed down or blocked

Without rate limiting, a shared admin password remains vulnerable to repeated guessing attempts.

## Design Principle

Rate limiting should be based on the best available client IP signal from a trusted reverse-proxy chain.

The key word is:

- trusted

This design must not assume that arbitrary client-supplied forwarding headers are safe to trust in every deployment.

## Trusted Reverse Proxy Model

The application should assume a deployment boundary like this:

- `simsexam` listens on `127.0.0.1:6080`
- only a local reverse proxy can connect to it
- the reverse proxy forwards trusted client IP headers

Under that model, `simsexam` can safely interpret specific proxy headers because the application is not directly exposed to the public network.

This assumption works with:

- Caddy
- Nginx
- Cloudflare in front of either one

It does not depend on any one proxy implementation.

## What Should Not Be Trusted

The application should not blindly trust:

- any incoming `X-Forwarded-For`
- any incoming `X-Real-IP`
- any incoming `CF-Connecting-IP`

unless the request reached the app through the trusted reverse-proxy boundary described above.

If the application were ever directly exposed to the public internet, those headers could be forged by clients.

## Client IP Resolution Strategy

Introduce a single helper, conceptually:

- `clientIPFromRequest(r)`

Its job is to normalize the best client IP for logging and rate limiting.

Recommended precedence:

1. `CF-Connecting-IP`
2. first valid IP from `X-Forwarded-For`
3. `X-Real-IP`
4. `RemoteAddr`

### Why This Order

- `CF-Connecting-IP` is the strongest signal when Cloudflare is in front
- `X-Forwarded-For` is the most common proxy chain convention
- `X-Real-IP` is often used by Nginx setups
- `RemoteAddr` is the fallback when no proxy headers are available

### Parsing Rules

The resolver should:

- trim whitespace
- split `X-Forwarded-For` on commas
- use the first valid public or private IP token in the list
- reject malformed values
- return an empty result only if no usable candidate exists

## Rate-Limit Scope

The first implementation only needs to protect:

- `POST /admin/login`

There is no need to rate-limit all admin routes in the first version.

## First-Version Storage Model

Use in-memory tracking.

Recommended key:

- resolved client IP string

Recommended tracked fields per key:

- `fail_count`
- `first_fail_at`
- `blocked_until`
- `last_fail_at`

This is acceptable because the current deployment model is:

- one process
- one host
- very low admin login volume

No database table is required for the first version.

## First-Version Behavior

Suggested rule set:

- allow normal login attempts initially
- if one IP accumulates 5 failed attempts within 10 minutes, block it for 10 minutes
- during the block window, return `429 Too Many Requests`
- a successful login clears the failure record for that IP

This gives a simple and understandable operator story.

## Response Behavior

### Failed Password

Recommended behavior:

- return the login page again
- show a generic invalid-password message

Do not reveal:

- whether the password was close
- how many attempts remain
- whether the system is using a special block threshold

### Blocked Client

Recommended behavior:

- return `429 Too Many Requests`
- show a generic cooldown message

Example message:

- `Too many failed login attempts. Please try again later.`

## Logging Recommendations

Every failed admin login attempt should log:

- resolved client IP
- `RemoteAddr`
- `CF-Connecting-IP` if present
- `X-Forwarded-For` if present
- `X-Real-IP` if present
- outcome: invalid password or blocked

Why this matters:

- it helps verify the deployment is forwarding the correct IP
- it helps operators diagnose whether rate limiting is using the right source address
- it creates a minimal audit trail for security review

The first version does not need a database-backed audit log. Standard application logs are enough.

## Proxy Compatibility

### Cloudflare + Caddy

Likely useful signal:

- `CF-Connecting-IP`

Fallbacks:

- `X-Forwarded-For`
- `RemoteAddr`

### Cloudflare + Nginx

Likely useful signal:

- `CF-Connecting-IP`

Fallbacks:

- `X-Forwarded-For`
- `X-Real-IP`
- `RemoteAddr`

### Local Reverse Proxy Only

Likely useful signal:

- `X-Forwarded-For`
or
- `X-Real-IP`

Fallback:

- `RemoteAddr`

## Why The Design Avoids Proxy-Specific Logic

The application should not contain logic such as:

- "if Caddy then do X"
- "if Nginx then do Y"

Instead, it should:

- read common proxy headers
- prefer stronger signals when present
- rely on the trusted proxy boundary

That keeps the design portable across deployment choices.

## Failure Modes To Consider

### Misconfigured Proxy Headers

If the reverse proxy does not pass the expected headers:

- multiple users may appear as the same IP
- rate limiting may become too aggressive or ineffective

### Direct Application Exposure

If the application is accidentally exposed directly:

- client-supplied forwarding headers may be spoofed
- IP-based rate limiting becomes less trustworthy

This is why the deployment assumption must remain:

- app bound to loopback only

### Process Restart

With in-memory tracking:

- rate-limit state resets on restart

This is acceptable for the first version.

## Future Improvements

Possible later enhancements:

- configurable thresholds
- structured audit log sink
- integration with reverse-proxy rate limiting
- persistent rate-limit state if deployment becomes multi-instance
- optional allowlist for trusted admin IP ranges

These are not required for the first version.

## Testing Considerations

Minimum tests for a future implementation:

- repeated failed logins from one resolved IP trigger blocking
- successful login clears prior failure state
- `CF-Connecting-IP` takes precedence when present
- `X-Forwarded-For` is parsed correctly
- malformed forwarding headers fall back safely
- blocked clients receive `429`

## Recommended Rollout

### Phase 1

- implement client IP resolver
- implement in-memory failed-login tracker
- apply limiter to `POST /admin/login`
- log failed attempts and blocked attempts

### Phase 2

- optionally add reverse-proxy-side rate limiting
- add production guidance for Caddy and Nginx forwarding headers

## Recommendation

`simsexam` should implement admin login rate limiting as:

- application-level
- IP-based
- trusted-reverse-proxy-aware
- proxy-agnostic

That gives meaningful brute-force resistance now without coupling the application to Caddy, Nginx, or any future proxy choice.
