import { defineConfig } from "vite";
import { resolve } from "path";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [
    tailwindcss({
      content: [
        "./internal/frontend/views/**/*.templ",
        "./internal/frontend/views/**/*.go",
        "./cmd/**/*.go",
      ],
    }),
  ],
  build: {
    outDir: "internal/frontend/static/dist",
    emptyOutDir: true,
    manifest: true,
    rollupOptions: {
      input: resolve(__dirname, "internal/frontend/static/main.js"),
      output: {
        entryFileNames: "assets/[name]-[hash].js",
        chunkFileNames: "assets/[name]-[hash].js",
        assetFileNames: "assets/[name]-[hash].[ext]",
      },
    },
  },
  server: {
    origin: "http://localhost:5173",
  },
});
