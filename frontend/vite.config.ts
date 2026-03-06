import { defineConfig } from "vite";

export default defineConfig({
  server: {
    port: 3005,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8081",
        changeOrigin: true
      },
      "/healthz": {
        target: "http://127.0.0.1:8081",
        changeOrigin: true
      },
      "/readyz": {
        target: "http://127.0.0.1:8081",
        changeOrigin: true
      }
    }
  }
});
