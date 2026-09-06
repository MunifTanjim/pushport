# CLI

The `pushport` binary is both the relay server and its management tool. `serve`
runs the relay; every other command manages it.

This page is a map. Run `pushport --help` or `pushport <command> --help` for the
full, always-current list of subcommands and flags.

## Talking to a server

Every command except `serve` needs a server URL and a token, most easily set once
via environment:

```sh
export PUSHPORT_BASE_URL="https://push.example.com"
export PUSHPORT_TOKEN="pat_..."        # an app token, or the admin token
```

- `--token` takes the admin token or an app token (`pat_…`).
- `--app <id>` targets an app for app-scoped commands. With an app token it
  defaults to `@app` (the token's own app); the admin token needs a concrete id.
- `-o json` prints the API's `data` payload for scripting:
  `pushport app list -o json | jq '.[].id'`.

## Commands

| Command      | What it does                                                                                 |
| ------------ | ------------------------------------------------------------------------------------------- |
| `serve`      | Run the relay (configured via [environment](/getting-started/configuration)).               |
| `app`        | Manage apps, the app token, and endpoint-key rotation.                                       |
| `creds`      | Set the app's transport credentials (APNs, FCM, WebPush).                                    |
| `instance`   | Manage instances, the `pit_` send-token holders.                                             |
| `turnstile`  | Configure Cloudflare Turnstile for [self-registration](/guide/instances#self-registration). |
| `usage-plan` | Manage [usage plans](/guide/usage-plans) for apps and instances.                            |
| `settings`   | Read and update runtime server settings (admin).                                            |

Run any command with `--help` for its subcommands and flags.
