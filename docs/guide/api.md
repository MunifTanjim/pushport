# HTTP API

Setup and management go through the [CLI](/guide/cli) or console. This page is the
reference for the two endpoints your own code calls at runtime: sending a push and
subscribing a device.

## Authentication

Tokens travel in the `Authorization: Bearer <token>` header:

| Token    | Prefix | Can do              |
| -------- | ------ | ------------------- |
| Admin    | none   | Everything.         |
| App      | `pat_` | Manage its own app. |
| Instance | `pit_` | Send pushes only.   |

## Response envelope

Every JSON response has a `request_id` (also in the `X-Request-Id` header) plus
exactly one of `data` or `error`:

```json
{ "request_id": "8Zk…", "data": { "endpoint": "…", "expires_at": 1795872000 } }
```

```json
{
  "request_id": "8Zk…",
  "error": {
    "code": "unauthorized",
    "message": "unauthorized",
    "status_code": 401
  }
}
```

`429` and some `503` responses carry a `Retry-After` header. `GET /healthz`
returns plain-text `ok`, no envelope.

## Send a Push

```
POST /push/{token}
```

Delivers an encrypted payload to the device behind a sealed push endpoint.
`{token}` is the opaque tail of the endpoint URL from
[subscribe](#subscribe-a-device).

**Auth:** the sending backend's instance token (`pit_…`). Its app must match the
endpoint's app, or the request is rejected with `403`.

**Headers:**

| Header             | Required | Description                                                                         |
| ------------------ | -------- | ----------------------------------------------------------------------------------- |
| `Content-Encoding` | yes      | Encoding of the ciphertext body (`aes128gcm`).                                      |
| `TTL`              | no       | Seconds the push stays relevant. Mapped to `apns-expiration` / FCM `ttl`.           |
| `Urgency`          | no       | `very-low` / `low` / `normal` / `high`. Mapped to `apns-priority` / FCM `priority`. |
| `Topic`            | no       | Collapse key. Mapped to `apns-collapse-id` / FCM `collapse_key`.                    |

**Body:** the raw `aes128gcm` ciphertext (RFC 8291 / RFC 8188), encrypted by the
backend to the device's keys. Capped at `PUSHPORT_MAX_PAYLOAD_BYTES` (3000 bytes
by default).

**Success:** `202 Accepted`, with an envelope carrying just `request_id` (no
`data`).

**Errors:**

| Status | Code                  | Meaning                                                                            |
| ------ | --------------------- | ---------------------------------------------------------------------------------- |
| `401`  | `unauthorized`        | Missing or invalid instance token.                                                 |
| `403`  | `forbidden`           | Token belongs to a different app than the endpoint.                                |
| `404`  | `not_found`           | Unknown or malformed endpoint token.                                               |
| `410`  | `gone`                | Endpoint expired, or the platform reported it permanently dead. Stop using it.     |
| `413`  | `payload_too_large`   | Body above the cap.                                                                |
| `429`  | `too_many_requests`   | Rate limit or quota hit; honor `Retry-After`.                                      |
| `502`  | `bad_gateway`         | Upstream platform rejected the push after retries.                                 |
| `503`  | `service_unavailable` | Upstream unavailable (may set `Retry-After`), or the transport has no credentials. |

## Subscribe a device

```
POST /apps/{app_id}/subscribe
Content-Type: application/json
```

Registers a device and returns a sealed push endpoint to send to. Unauthenticated,
rate-limited per app (the app's subscribe limit applies).

**Body:**

| Field       | Required | Description                                                                                                                         |
| ----------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `transport` | yes      | `apns`, `fcm`, or `webpush`.                                                                                                        |
| `token`     | yes      | The device's native token: APNs/FCM registration token, or the WebPush subscription endpoint URL.                                   |
| `ttl`       | no       | Endpoint lifetime as a Go duration string (e.g. `360h`). Clamped to 12h–45d; defaults to the server's `PUSHPORT_PUSH_ENDPOINT_TTL`. |
| `sandbox`   | no       | APNs only. `true` routes the endpoint through the APNs sandbox environment; defaults to `false` (production).                       |

**Success:** `201 Created`:

```json
{
  "request_id": "8Zk…",
  "data": {
    "endpoint": "https://push.example.com/push/a1b2c3…",
    "expires_at": 1795872000
  }
}
```

`endpoint` is the URL to [send](#send-a-push) to; `expires_at` is a Unix timestamp.

**Errors:** `400 bad_request` (invalid `transport`, `token`, or `ttl`),
`404 not_found` (unknown app), `415 unsupported_media_type` (non-JSON body),
`429 too_many_requests` (rate limited).

## Error codes

| `code`                   | Status | Typical cause                                      |
| ------------------------ | ------ | -------------------------------------------------- |
| `bad_request`            | 400    | Malformed body or parameters.                      |
| `unauthorized`           | 401    | Missing/invalid token.                             |
| `forbidden`              | 403    | Token not valid for this resource.                 |
| `not_found`              | 404    | Unknown app, instance, or endpoint.                |
| `gone`                   | 410    | Expired or permanently dead endpoint.              |
| `payload_too_large`      | 413    | Push body above the cap.                           |
| `unsupported_media_type` | 415    | `Content-Type` is not `application/json`.          |
| `too_many_requests`      | 429    | Rate limit or quota hit; honor `Retry-After`.      |
| `bad_gateway`            | 502    | Upstream platform rejected the push.               |
| `service_unavailable`    | 503    | Upstream unavailable, or transport not configured. |
