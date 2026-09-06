# Usage Plans

Usage plans set rate limits and daily quotas on traffic. An over-limit request
gets `429 Too Many Requests` with a `Retry-After` header, and never reaches the
platform.

Plans come in two kinds:

- **App plan** (admin): caps an app's pushes, plus its
  [self-registration](/guide/instances#self-registration) and
  [subscribe](/guide/instances#subscribing-a-device) endpoints.
- **Instance plan** (app owner): caps one instance's pushes.

For each, the relay uses the assigned plan, falling back to an auto-created
default. So an admin can cap an entire app, and an app owner can cap each
self-hoster's backend beneath that.

Limits default to `0` (unlimited), so only the ones you set take effect; daily
quotas reset per UTC day. Create and assign plans with the
[`usage-plan` CLI](/guide/cli) or the console. See
[configuration](/getting-started/configuration#self-registration-and-subscribe-limits)
for the registration and subscribe flags.
