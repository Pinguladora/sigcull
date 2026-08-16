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

  These assert what NVDA actually announces, which axe/keyboard cannot: the two
  labelled nav landmarks, the main region, and the skip link. Treat as a starting
  point — spoken-phrase strings vary by NVDA version, so adjust expectations to
  the real log on first run.
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

  test("announces the skip link first", async ({ page, nvda }) => {
    await page.goto("/docs/authorities/");
    // The skip link is the first focusable element (first child of <body>), so
    // the first Tab moves focus to it. Browse-mode next() lands on the brand link
    // instead, so drive a real Tab press.
    await nvda.press("Tab");
    const spoken = (await nvda.lastSpokenPhrase()).toLowerCase();
    await nvda.stop();
    expect(spoken).toContain("skip to content"); // DOM link text, not a role word
  });
});
