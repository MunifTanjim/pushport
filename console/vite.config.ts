import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import sri from "vite-plugin-sri-gen";

// The console is a standalone SPA hosted on Cloudflare Pages at the site root;
// it reaches the API cross-origin via VITE_API_BASE_URL. In dev the client uses
// an /api base (see api/client.ts) and the Vite server proxies that single
// prefix to the local pushport server on :8080, stripping /api so requests stay
// same-origin and no CORS setup is needed locally.
export default defineConfig({
  base: "/",
  // sri() last so it hashes the final emitted script/style tags.
  plugins: [react(), tailwindcss(), sri()],
  server: {
    port: 3000,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
});
