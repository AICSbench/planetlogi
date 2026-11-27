import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: "autoUpdate",
      includeAssets: ["icons/planetlogi-icon.svg", "_redirects"],
      manifest: {
        name: "PlanetLogi — Cosmic Math Adventures",
        short_name: "PlanetLogi",
        description:
          "Galaxy-themed bilingual elementary math learning platform for Grades 1–6.",
        theme_color: "#0B1026",
        background_color: "#0B1026",
        display: "standalone",
        orientation: "any",
        start_url: "/",
        scope: "/",
        icons: [
          {
            src: "/icons/planetlogi-icon.svg",
            sizes: "any",
            type: "image/svg+xml",
            purpose: "any maskable",
          },
        ],
      },
      workbox: {
        globPatterns: ["**/*.{js,css,html,svg,png,woff2}"],
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            urlPattern: ({ url }) =>
              url.origin === self.location.origin &&
              /^\/api\/v1\/(training|academy|stories)\//.test(url.pathname),
            handler: "NetworkFirst",
            options: {
              cacheName: "planetlogi-api-cache",
              expiration: { maxEntries: 60, maxAgeSeconds: 60 * 60 * 24 },
            },
          },
        ],
      },
    }),
  ],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
