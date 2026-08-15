import { defineConfig, devices } from "@playwright/test";

// Tests run against the BUILT site in public/ (production output, no livereload
// JS), served by a throwaway static server. Never against `hugo server`.
export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: true,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:8099",
    trace: "on-first-retry",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: {
    // Call the binary directly; going through `pnpm exec` hangs when Playwright
    // spawns it (pnpm waits on something with no TTY).
    command: "node_modules/.bin/http-server public -p 8099 -a 127.0.0.1 -c-1 --silent",
    url: "http://127.0.0.1:8099/",
    reuseExistingServer: true,
    timeout: 30_000,
  },
});
