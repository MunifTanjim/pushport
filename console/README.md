# pushport console

Standalone admin console (Vite + React) for pushport, hosted on
Cloudflare Pages. It talks to the pushport API cross-origin.

## Local development

    pnpm --filter console dev

The Vite dev server on :3000 proxies the `/api` prefix to a local pushport
server on :8080 (stripping `/api`), so `VITE_API_BASE_URL` can stay unset
locally.

## Build

    pnpm --filter console build   # outputs to console/dist

## Cloudflare Pages

- Build command: `pnpm --filter console build`
- Build output directory: `console/dist` (see `wrangler.toml`)
- Build environment variable (optional): `VITE_API_BASE_URL` = the pushport
  API's public URL (e.g. `https://api.example.com`). Production builds default
  to `https://pushport.muniftanjim.dev` when unset.

Deploy via the dashboard or:

    pnpm --filter console build
    pnpm dlx wrangler pages deploy console/dist

## API requirement

The pushport server must allow the console origin for CORS:

    PUSHPORT_CORS_ORIGIN=https://<your-pages-domain>

Multiple origins are comma-separated (e.g. production + preview).
