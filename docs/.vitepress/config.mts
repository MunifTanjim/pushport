import { copyFileSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitepress";

const hostname = "https://docs.pushport.muniftanjim.dev";

// Served at the site root so `curl https://docs.pushport.muniftanjim.dev/install.sh` works
const installScriptPath = fileURLToPath(
  new URL("../../scripts/install.sh", import.meta.url),
);

export default defineConfig({
  cleanUrls: true,
  lastUpdated: true,

  // Follow OS prefers-color-scheme by default, with a persisted navbar toggle.
  appearance: true,

  title: "PushPort",
  description:
    "Push Notification Relay for self-hosted apps. Supports APNs, FCM, and WebPush: end-to-end encrypted payloads.",

  head: [
    [
      "link",
      {
        rel: "icon",
        type: "image/svg+xml",
        href: "/logo.svg",
      },
    ],
  ],

  sitemap: {
    hostname,
  },

  buildEnd(siteConfig) {
    copyFileSync(installScriptPath, `${siteConfig.outDir}/install.sh`);
  },

  vite: {
    plugins: [
      {
        name: "serve-install-script",
        configureServer(server) {
          server.middlewares.use("/install.sh", (_req, res) => {
            res.setHeader("Content-Type", "text/plain");
            res.end(readFileSync(installScriptPath));
          });
        },
      },
    ],
  },

  transformHead({ pageData, siteData }) {
    const isHome = pageData.frontmatter.layout === "home";
    const raw = (pageData.frontmatter.title as string) || pageData.title;
    const title = raw && !isHome ? `${raw} | PushPort` : "PushPort";
    const description =
      (pageData.frontmatter.description as string) || siteData.description;
    const path = pageData.relativePath
      .replace(/(^|\/)index\.md$/, "$1")
      .replace(/\.md$/, "");
    const url = `${hostname}/${path}`;
    const image = `${hostname}/og-image.png`;
    return [
      ["meta", { property: "og:type", content: "website" }],
      ["meta", { property: "og:site_name", content: "PushPort" }],
      ["meta", { property: "og:title", content: title }],
      ["meta", { property: "og:description", content: description }],
      ["meta", { property: "og:url", content: url }],
      ["meta", { property: "og:image", content: image }],
      ["meta", { property: "og:image:width", content: "1200" }],
      ["meta", { property: "og:image:height", content: "630" }],
      ["meta", { name: "twitter:card", content: "summary_large_image" }],
      ["meta", { name: "twitter:title", content: title }],
      ["meta", { name: "twitter:description", content: description }],
      ["meta", { name: "twitter:image", content: image }],
    ];
  },

  themeConfig: {
    logo: "/logo.svg",

    nav: [
      { text: "Getting Started", link: "/getting-started/introduction" },
      { text: "Guide", link: "/guide/apps" },
      { text: "Console", link: "https://console.pushport.muniftanjim.dev" },
    ],

    sidebar: [
      {
        text: "Getting Started",
        items: [
          { text: "Introduction", link: "/getting-started/introduction" },
          { text: "Installation", link: "/getting-started/installation" },
          { text: "Configuration", link: "/getting-started/configuration" },
        ],
      },
      {
        text: "Guide",
        items: [
          { text: "Apps & Credentials", link: "/guide/apps" },
          { text: "Instances & Devices", link: "/guide/instances" },
          { text: "Sending Pushes", link: "/guide/sending" },
          { text: "Usage Plans", link: "/guide/usage-plans" },
          { text: "CLI", link: "/guide/cli" },
          { text: "HTTP API", link: "/guide/api" },
        ],
      },
    ],

    socialLinks: [
      {
        icon: "github",
        link: "https://github.com/MunifTanjim/pushport",
      },
      {
        icon: "discord",
        link: "https://go.muniftanjim.dev/discord",
      },
      {
        icon: "buymeacoffee",
        link: "https://buymeacoffee.com/muniftanjim",
      },
      {
        icon: "patreon",
        link: "https://www.patreon.com/muniftanjim",
      },
    ],

    editLink: {
      pattern: "https://github.com/MunifTanjim/pushport/edit/main/docs/:path",
    },

    footer: {
      copyright: "© 2026 Munif Tanjim",
    },

    search: {
      provider: "local",
    },
  },
});
