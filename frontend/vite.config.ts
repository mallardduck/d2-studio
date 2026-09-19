import { defineConfig, type Plugin } from "vite";
import { closeSync, openSync } from "node:fs";
import { join } from "node:path";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

// Vite empties dist/ on every build, which removes dist/.gitkeep. That file
// keeps the directory in git so `//go:embed all:frontend/dist` compiles on a
// fresh clone, so put it back after each build.
function keepDistPlaceholder(): Plugin {
  let outDir = "dist";
  return {
    name: "keep-dist-placeholder",
    apply: "build",
    configResolved(config) {
      outDir = config.build.outDir;
    },
    closeBundle() {
      closeSync(openSync(join(outDir, ".gitkeep"), "a"));
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), tailwindcss(), wails("./bindings"), keepDistPlaceholder()],
});
