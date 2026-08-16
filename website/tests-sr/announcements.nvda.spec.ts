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
    mise run site:sr        (or: pnpm exec playwright test --config playwright.sr.config.ts)

  NVDA and the guidepup addon must be installed once before the first run:
    pnpm dlx @guidepup/setup setup     (configures the OS, once per machine)
    pnpm dlx @guidepup/setup install   (installs NVDA and the addon, once per project)
  On CI, use the guidepup/setup-action step instead.

  This asserts what NVDA actually announces, which axe cannot: the two labelled
  nav landmarks are named distinctly and the main content is reached. Spoken
  strings vary by NVDA version and locale, so it keys off page content (aria-label
  values, headings), never localized role words like "main" or "link". The skip
  link is a keyboard affordance whose behaviour is covered deterministically by
  tests/keyboard.spec.ts in the Linux gate. NVDA browse navigation does not
  surface the off-screen fixed-position link, so it is not asserted here.
*/

test.describe("NVDA announcements", () => {
  // Assertions key off page content (aria-label values, headings, link text),
  // which reads the same whatever NVDA's UI locale is. Role words like "main" or
  // "link" are localized (Spanish "principal", "enlace"), so they are avoided.
  test("names the two nav landmarks distinctly and reaches main", async ({ page, nvda }) => {
    await page.goto("/docs/authorities/");
    await nvda.navigateToWebContent();
    for (let i = 0; i < 80; i++) await nvda.next();
    const log = (await nvda.spokenPhraseLog()).join(" | ").toLowerCase();
    await nvda.stop();

    expect(log).toContain("documentation"); // main menu nav aria-label
    expect(log).toContain("on this page"); // TOC nav aria-label
    expect(log).toContain("authorities"); // the <main> h1, so main content was reached
  });
});
