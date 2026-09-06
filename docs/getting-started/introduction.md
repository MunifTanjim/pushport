# Introduction

**Push Notification Relay for self-hosted apps.**

An open-source app is published once, to the App Store, the Play Store, and the
web. But its backend is self-hosted by many people. The platform push credentials
(an APNs key, an FCM service account, a VAPID keypair) belong to the _app project_,
not the self-hosters. So those backends can't deliver pushes on their own.

PushPort is the relay in between: it holds the credentials once and
lets any number of backends deliver pushes through it.

## Highlights

- **Three transports**: APNs, FCM, and WebPush, behind one endpoint format.
- **Simple send API**: POST an encrypted payload with TTL, Urgency, and Topic
  headers, authenticated by an instance token.
- **End-to-end encryption**: RFC 8291 + RFC 8188 `aes128gcm`; the relay
  carries ciphertext it cannot decrypt.
- **Sealed push endpoints**: each device gets an unguessable, self-describing
  endpoint URL. No per-device registration table.
- **Multi-tenant**: several apps coexist on one relay, credentials and quotas
  isolated per app.
- **Self-service instances**: self-hosters register their backend and get a
  token; sign-ups can be gated by Cloudflare Turnstile.
- **Usage plans**: per-min and daily push limits per app and per instance,
  enforced before any platform call.

## How it fits together

PushPort recognizes four roles:

- **Admin**: runs the relay. Holds the `PUSHPORT_ADMIN_TOKEN`, creates
  apps, and manages [app usage plans](/guide/usage-plans).
- **App**: one published app, owned by the app project. Holds the platform
  push credentials and an app token (`pat_…`).
- **Instance**: one self-hosted backend of that app. Holds an instance token
  (`pit_…`) and uses it to send pushes.
- **Device**: one install of the app. Subscribes with its native push token
and receives a sealed [push endpoint](/guide/instances#subscribing-a-device).

A push then travels like this:

1. The device [subscribes](/guide/instances#subscribing-a-device) and receives
   its sealed push endpoint.
2. The device shares the push endpoint plus its content-encryption keys
   (`p256dh` + `auth`) with the backend it trusts. The private key never leaves
   the device, and the keys never touch the relay.
3. The backend [sends](/guide/sending): it encrypts the payload with RFC 8291
   and POSTs it to the endpoint with its instance token.
4. PushPort authenticates the token, enforces
   [usage plans](/guide/usage-plans), opens the sealed push endpoint, and relays
   to APNs, FCM, or WebPush service, adding the platform
   credentials and the VAPID signature as needed.

## What the relay is not

- **Not a push service.** APNs, FCM, and WebPush services already do the
  stateful work: store-and-forward for offline devices, wake channels.
  PushPort is the application server in front of them: it forwards
  synchronously and returns the upstream result.
- **Not a message queue.** There is no durable delivery queue and no delivery
  receipts; a dead endpoint comes back as `410 Gone` so the backend can drop
  it.
- **Not a plaintext courier.** The relay holds _transport_ credentials only.
  Content keys stay between the device and the backend.
