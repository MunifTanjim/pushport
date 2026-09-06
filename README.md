# PushPort

**Push Notification Relay for self-hosted apps.**

An open-source app is published once, to the App Store, the Play Store, and the
web. But its backend is self-hosted by many people. The platform push
credentials (an APNs key, an FCM service account, a VAPID keypair) belong to the
_app project_, not the self-hosters, so those backends can't deliver pushes on
their own.

PushPort is the relay in between: it holds the credentials once and
lets any number of backends deliver pushes through it.

> [!IMPORTANT]
> 📖 **Read the documentation at [docs.pushport.muniftanjim.dev](https://docs.pushport.muniftanjim.dev)**.

## Highlights

- **Three transports**: APNs, FCM, and WebPush behind one endpoint.
- **Simple send API**: POST an encrypted payload, authenticated by an instance token.
- **End-to-end encryption**: RFC 8291 + RFC 8188 `aes128gcm`; the relay never sees plaintext.
- **Sealed endpoints**: unguessable per-device URLs, no registration table.
- **Multi-tenant**: many apps on one relay, isolated credentials and quotas.
- **Self-service instances**: backends self-register, optionally gated by Turnstile.
- **Usage plans**: per-minute and daily limits, enforced before any platform call.

## Quick start

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

## License

Licensed under the MIT License. Check the [LICENSE](./LICENSE) file for details.
