---
layout: home

hero:
  name: PushPort
  text: Push Notification Relay for self-hosted apps
  tagline: One relay holds the transport credentials, so every self-hosted backend can deliver.
  image:
    src: /logo.svg
    alt: PushPort
  actions:
    - theme: brand
      text: What is PushPort?
      link: /getting-started/introduction
    - theme: alt
      text: Run the server
      link: /getting-started/installation
    - theme: alt
      text: GitHub
      link: https://github.com/MunifTanjim/pushport

features:
  - icon:
      src: "data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%20fill%3D%22none%22%20stroke%3D%22%23111111%22%20stroke-width%3D%222%22%20stroke-linecap%3D%22round%22%20stroke-linejoin%3D%22round%22%3E%3Crect%20width%3D%2220%22%20height%3D%2216%22%20x%3D%222%22%20y%3D%224%22%20rx%3D%222%22%2F%3E%3Cpath%20d%3D%22M22%207l-8.97%205.7a1.94%201.94%200%200%201-2.06%200L2%207%22%2F%3E%3C%2Fsvg%3E"
      width: "24"
      height: "24"
    title: One Binary, Every Transport
    details: One Go binary, embedded SQLite, one send API for APNs, FCM, and WebPush.
    link: /getting-started/introduction
  - icon:
      src: "data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%20fill%3D%22none%22%20stroke%3D%22%23111111%22%20stroke-width%3D%222%22%20stroke-linecap%3D%22round%22%20stroke-linejoin%3D%22round%22%3E%3Crect%20width%3D%2218%22%20height%3D%2211%22%20x%3D%223%22%20y%3D%2211%22%20rx%3D%222%22%20ry%3D%222%22%2F%3E%3Cpath%20d%3D%22M7%2011V7a5%205%200%200%201%2010%200v4%22%2F%3E%3C%2Fsvg%3E"
      width: "24"
      height: "24"
    title: Encrypted and Sealed
    details: End-to-end encrypted, so the relay never reads your payloads. Each device gets its own unguessable endpoint.
    link: /guide/sending
  - icon:
      src: "data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%20fill%3D%22none%22%20stroke%3D%22%23111111%22%20stroke-width%3D%222%22%20stroke-linecap%3D%22round%22%20stroke-linejoin%3D%22round%22%3E%3Cellipse%20cx%3D%2212%22%20cy%3D%225%22%20rx%3D%229%22%20ry%3D%223%22%2F%3E%3Cpath%20d%3D%22M3%205V19A9%203%200%200%200%2021%2019V5%22%2F%3E%3Cpath%20d%3D%22M3%2012A9%203%200%200%200%2021%2012%22%2F%3E%3C%2Fsvg%3E"
      width: "24"
      height: "24"
    title: Multi-Tenant with Guardrails
    details: Many isolated apps on one relay, each with its own rate limits and quotas.
    link: /guide/usage-plans
---

## Get started

```sh
# write the two required secrets to .env (bare KEY=value, no quotes)
cat > .env <<EOF
PUSHPORT_ADMIN_TOKEN=$(openssl rand -hex 32)
PUSHPORT_SECRET=$(openssl rand -base64 32)
EOF

# run the relay (SQLite lives in ./pushport/data on the host)
mkdir -p pushport/data
docker run -d --name pushport \
  -p 8080:8080 -v "$(pwd)/pushport/data:/app/data" \
  --env-file .env \
  muniftanjim/pushport:latest
```

The full walkthrough is in [Installation](/getting-started/installation).

## How a push travels

<div class="flow">
<a class="flow-step" href="/guide/apps">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 16h6"/><path d="M19 13v6"/><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l2-1.15"/><path d="M3.3 7 12 12l8.7-5"/><path d="M12 22V12"/></svg>
<h3>1 · Register</h3>
<p>The admin creates an <strong>app</strong> and sets the transport credentials: APNs <code>.p8</code>, FCM service account, VAPID private key.</p>
</a>
<div class="flow-arrow" aria-hidden="true">→</div>
<a class="flow-step" href="/guide/instances">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/></svg>
<h3>2 · Connect</h3>
<p>Self-hosted backends register as <strong>instances</strong> and get a token. Devices subscribe and receive a sealed <strong>push endpoint</strong>.</p>
</a>
<div class="flow-arrow" aria-hidden="true">→</div>
<a class="flow-step" href="/guide/sending">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>
<h3>3 · Push</h3>
<p>The backend POSTs an encrypted payload to the push endpoint. PushPort relays to APNs, FCM, or WebPush service.</p>
</a>
</div>
