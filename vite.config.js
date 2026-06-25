import { defineConfig } from "vite"
import { resolve } from "path"
import tailwindcss from "@tailwindcss/vite"

export default defineConfig({
  plugins: [
    tailwindcss({
      content: [
        "./internal/frontend/view/**/*.templ",
        "./internal/frontend/view/**/*.go",
        "./internal/parser/**/*.go"
      ]
    })
  ],
  build: {
    outDir: "internal/frontend/static/dist",
    emptyOutDir: true,
    manifest: true,
    rollupOptions: {
      input: resolve(__dirname, "internal/frontend/static/main.ts"),
      output: {
        entryFileNames: "assets/[name]-[hash].js",
        chunkFileNames: "assets/[name]-[hash].js",
        assetFileNames: "assets/[name]-[hash].[ext]"
      }
    }
  },
  server: {
    origin: "http://localhost:5173"
  }
})
