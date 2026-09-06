// @guidepup/playwright is CommonJS with no statically-detectable named exports,
// and this project is "type": "module", so a named import of nvdaTest fails
// under Playwright's ESM loader. Import the default and destructure it. expect
// is not exported by guidepup; it comes from @playwright/test like the other specs.
import guidepup from "@guidepup/playwright";
import { expect } from "@playwright/test";

const { nvdaTest: test } = guidepup;

/*
  Real-NVDA screen-reader tests. Windows + a running NVDA only — @guidepup/playwright
  drives the actual AT, so these are deliberately kept OUT of the Linux gate
  (playwright.config.ts testDir is ./tests). Run on Windows with:
    mise run site-astro:sr   (or: pnpm exec playwright test --config playwright.sr.config.ts)

  NVDA and the guidepup addon must be installed once before the first run:
    pnpm dlx @guidepup/setup setup     (configures the OS, once per machine)
    pnpm dlx @guidepup/setup install   (installs NVDA and the addon, once per project)
  On CI, use the guidepup/setup-action step instead.

  This asserts what NVDA actually announces, which axe cannot: that a keyboard
  reader traversing the page hears the site navigation, the page body, and its
  section headings. It keys off page CONTENT (sidebar link text, the main
  heading, a section heading), never localized role words. Starlight labels its
  landmarks with role words ("Main", "On this page") that NVDA localizes, so
  those are avoided. The skip link is a keyboard affordance covered
  deterministically by tests/keyboard.spec.ts in the Linux gate.
*/

test.describe("NVDA announcements", () => {
  test("announces the site nav, the main heading, and a section heading", async ({
    page,
    nvda,
  }) => {
    await page.goto("/authorities/");
    await nvda.navigateToWebContent();
    for (let i = 0; i < 100; i++) await nvda.next();
    const log = (await nvda.spokenPhraseLog()).join(" | ").toLowerCase();
    await nvda.stop();

    expect(log).toContain("getting started"); // a sidebar nav link, so the nav was reached
    expect(log).toContain("authorities"); // the <main> h1, so main content was reached
    expect(log).toContain("keyless"); // a section heading on the page, announced in body/TOC
  });
});
