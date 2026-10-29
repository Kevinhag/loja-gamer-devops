import { defineConfig } from "vitest/config";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";

// Em desenvolvimento o Vite repassa as chamadas para o backend Go.
const backend = process.env.BACKEND_URL ?? "http://localhost:8080";

export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  server: {
    proxy: {
      "/api": backend,
      "/images": backend,
      "/healthz": backend,
      "/metrics": backend,
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test-setup.js"],
    include: ["src/**/*.test.js"],
    reporters: ["default", "junit"],
    outputFile: { junit: "./test-results/junit.xml" },
    coverage: {
      provider: "v8",
      include: ["src/**/*.{js,svelte}"],
      exclude: ["src/**/*.test.js", "src/main.js", "src/test-setup.js"],
      reporter: ["text-summary", "cobertura", "html"],
      reportsDirectory: "./coverage",
    },
  },
});
