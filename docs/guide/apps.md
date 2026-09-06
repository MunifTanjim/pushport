# Apps & Credentials

An **app** is one published application, the top-level tenant on a relay: its
platform push credentials, app token, instances, and quotas all hang off it.

Apps are created with the admin token, then managed by it or by the app's own
token. See the [CLI reference](/guide/cli) for every flag.

## Creating an app

```sh
pushport app create --name "My App"
# ID         NAME     TOKEN (shown once)
# a1b2c3...  My App   pat_...
```

The **app token** (`pat_` prefix) is shown once. It authenticates every
app-scoped management call (credentials, instances, Turnstile, rotation); store
it securely and [rotate it](#rotating-tokens-and-keys) if it leaks.

All app-scoped CLI commands accept `--app <id>`; with an app token you can
leave it as the default `@app` and the token resolves the app for you.

## Visibility: public vs private

An app is **private** by default. Only the admin and the app's own token
can create instances for it.

Setting an app **public** opens self-registration: anyone can
[register an instance](/guide/instances#self-registration) and receive an
instance token. Gate it with [Turnstile](#turnstile) to keep bots out.

```sh
pushport app update --public
```

## Transport Credentials

These credentials authenticate delivery to each transport (APNs, FCM, WebPush).
They're encrypted at rest and used only to authenticate with each transport.

| Transport | What you upload                                                            | Where to get it                                           |
| --------- | -------------------------------------------------------------------------- | --------------------------------------------------------- |
| `apns`    | `.p8` key, key id, Apple team id, topic (bundle id), sandbox vs production | Apple Developer → Keys                                    |
| `fcm`     | Service-account JSON, project id                                           | Google Cloud → Service Accounts                           |
| `webpush` | VAPID private key, subject (`mailto:` or URL)                              | Generate locally, e.g. `npx web-push generate-vapid-keys` |

```sh
export PUSHPORT_TOKEN="pat_..."   # app token

# APNs (the .p8 file can be passed as @path/to/AuthKey.p8)
pushport creds set apns \
  --key-p8 @AuthKey_AB12CD34.p8 \
  --key-id AB12CD34 \
  --team-id TEAM1234 \
  --topic com.example.myapp \
  --production

# FCM
pushport creds set fcm \
  --service-account @firebase-service-account.json \
  --project-id my-app-123

# WebPush (public key is derived from the private key)
pushport creds set webpush \
  --vapid-private "iK9Z..." \
  --subject "mailto:admin@example.com"

pushport creds list
```

A transport only works once its credentials are set; a push routed to an
unconfigured transport returns `503 transport not configured`. Remove
credentials with `pushport creds rm apns|fcm|webpush`.

## Rotating Tokens and Keys

### App Token

```sh
pushport app rotate-token
```

Issues a fresh `pat_` token (shown once) and invalidates the old one
immediately.

### Endpoint Key

Every push endpoint is sealed with the app's _endpoint key_. Rotation is
graceful by default: new endpoints use the new key, and old ones keep working
until they expire (max 45 days) since the previous key stays in the keyring.

```sh
# graceful rotation (old endpoints keep working until expiry)
pushport app rotate-endpoint-key

# emergency: also invalidate everything sealed with the old key, now
pushport app rotate-endpoint-key --revoke-old
```

Use `--revoke-old` only for break-glass (a leaked endpoint set): every device
sealed under the old key must re-register, and blast radius is capped to this
one app.
