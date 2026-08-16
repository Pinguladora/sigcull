import { defineConfig, devices } from "@playwright/test";

// Real-NVDA screen-reader suite (Windows only). Separate from playwright.config.ts
// so the Linux gate never loads @guidepup/playwright (it throws at import when no
// real screen reader is available). Run on Windows: `mise run site:sr`.
export default defineConfig({
  testDir: "./tests-sr",
  fullyParallel: false,
  workers: 1, // one screen-reader instance at a time
  forbidOnly: true,
  timeout: 120_000,
  reporter: [["list"]],
  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://127.0.0.1:8099",
    headless: false, // a screen reader needs a real, focused browser window
  },
  webServer: {
    // Serves the built site. Adjust for your Windows shell if needed.
    command: "pnpm exec http-server public -p 8099 -a 127.0.0.1 -c-1 --silent",
    url: "http://127.0.0.1:8099/",
    reuseExistingServer: true,
    timeout: 30_000,
  },
});
