# Installation

PushPort ships as a single static binary and a container image.

Use the container to deploy. Grab the binary when you're running locally, on a
box without Docker, or you just want the CLI pointed at a remote server.

## Image

Images are published to both Docker Hub and the GitHub Container Registry:

```
muniftanjim/pushport
ghcr.io/muniftanjim/pushport
```

In production, pin a version tag like `0.0.1`. The `latest` tag points
to the newest release.

## Binary

Binaries for Linux and macOS (`amd64`/`arm64`) are attached to each
[GitHub release](https://github.com/MunifTanjim/pushport/releases). The install
script fetches the one for your platform into `~/.local/bin`:

```sh
curl -fsSL https://pushport.muniftanjim.dev/install.sh | bash
```

Set `INSTALL_DIR` to install elsewhere (make sure it's on your `PATH`):

```sh
curl -fsSL https://pushport.muniftanjim.dev/install.sh | INSTALL_DIR=/usr/local/bin bash
```

Or install from source with Go:

```sh
go install github.com/MunifTanjim/pushport@latest   # -> $(go env GOPATH)/bin
```

The same binary runs the server (`pushport serve`) and is the CLI for every
other command.

## Required Secrets

Two environment variables are mandatory. The server refuses to start without
them:

| Variable               | What it is                                         |
| ---------------------- | -------------------------------------------------- |
| `PUSHPORT_ADMIN_TOKEN` | Bearer token for admin API calls and CLI commands. |
| `PUSHPORT_SECRET`      | Master secret, base64 of exactly 32 random bytes.  |


```sh
# bare KEY=value, no quotes (works with --env-file and Compose)
cat > .env <<EOF
PUSHPORT_ADMIN_TOKEN=$(openssl rand -hex 32)
PUSHPORT_SECRET=$(openssl rand -base64 32)
EOF
```

::: warning Treat PUSHPORT_SECRET as permanent
Everything sealed with it (transport credentials, app and instance tokens,
push endpoints) becomes unreadable if you lose it. To rotate, see
[Configuration: Secret Rotation](/getting-started/configuration#secret-rotation).
:::

## Run with `docker run`

The image stores its SQLite database under `/app/data`. Mount a host directory
there so state survives restarts (and the db file stays where you can see it):

```sh
mkdir -p pushport/data
docker run -d --name pushport \
  -p 8080:8080 \
  -v "$(pwd)/pushport/data:/app/data" \
  --env-file .env \
  -e PUSHPORT_BASE_URL="https://pushport.example.com" \
  muniftanjim/pushport:latest
```

## Run with Docker Compose

```yaml
# compose.yaml
services:
  pushport:
    image: muniftanjim/pushport:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./pushport/data:/app/data
    environment:
      PUSHPORT_ADMIN_TOKEN: ${PUSHPORT_ADMIN_TOKEN}
      PUSHPORT_SECRET: ${PUSHPORT_SECRET}
      PUSHPORT_BASE_URL: https://pushport.example.com
```

Put the two secrets in a `.env` file next to the compose file, then:

```sh
docker compose up -d
```

## Run the Binary

Load the [two secrets](#required-secrets) from `.env` and start it:

```sh
env $(cat .env) pushport serve
```

Listens on `:8080` (`PUSHPORT_ADDR`); database at `PUSHPORT_DATABASE_URI`
(default `sqlite://./data/pushport.db`). Set `PUSHPORT_BASE_URL` behind a proxy.

## First run

Check it's alive:

```sh
curl http://localhost:8080/healthz
# ok
```

The image doubles as the CLI. Run it against the server, sharing its network so
the default `localhost:8080` resolves, and pass the same `.env` for the token:

```sh
docker run --rm --network container:pushport --env-file .env \
  muniftanjim/pushport:latest app create --name "My App"
# ID         NAME     TOKEN (shown once)
# a1b2c3...  My App   pat_...
```

The app token (`pat_…`) is shown **once**. Store it in your app
project's secret manager. From here, the typical setup order is:

1. [Upload transport credentials](/guide/apps#transport-credentials) for the
   transports you support.
2. [Register instances](/guide/instances#registering-an-instance) (or open
   public self-registration) so backends can get instance tokens.
3. Point your app's devices at
   [`POST /apps/{app_id}/subscribe`](/guide/instances#subscribing-a-device).

::: tip Run the CLI from anywhere
Point the CLI at the server with `--base-url`/`PUSHPORT_BASE_URL` and a token to
manage the relay from your own machine. See the [CLI reference](/guide/cli).
:::

## Running behind a Reverse Proxy

Set `PUSHPORT_BASE_URL` to the public URL. It is baked into every push endpoint
PushPort mints. Terminate TLS at your proxy (Caddy, nginx, etc.) and forward to
the container's published port.

Change it later and only new endpoints use the new URL, so keep the old address
routing to the server until devices re-register.

## Upgrading

State lives in the SQLite file; the schema lives in the binary. No migration step.

**Container:** pull and recreate; the volume keeps the database:

```sh
docker compose pull && docker compose up -d --force-recreate
```

**Binary:** downloads the latest release and swaps it in place:

```sh
pushport upgrade
```
