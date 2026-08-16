import { nvdaTest as test, expect } from "@guidepup/playwright";

/*
  Real-NVDA screen-reader tests. Windows + a running NVDA only — @guidepup/playwright
  drives the actual AT, so these are deliberately kept OUT of the Linux gate
  (playwright.config.ts testDir is ./tests). Run on Windows with:
    mise run site:sr        (or: pnpm exec playwright test --config playwright.sr.config.ts)

  These assert what NVDA actually announces, which axe/keyboard cannot: the two
  labelled nav landmarks, the main region, and the skip link. Treat as a starting
  point — spoken-phrase strings vary by NVDA version, so adjust expectations to
  the real log on first run.
*/

test.describe("NVDA announcements", () => {
  test("names the two nav landmarks distinctly and reaches main", async ({ page, nvda }) => {
    await page.goto("/docs/authorities/");
    await nvda.navigateToWebContent();
    for (let i = 0; i < 80; i++) await nvda.next();
    const log = (await nvda.spokenPhraseLog()).join(" | ").toLowerCase();
    await nvda.stop();

    expect(log).toContain("documentation"); // main menu nav aria-label
    expect(log).toContain("on this page"); // TOC nav aria-label
    expect(log).toContain("main"); // main landmark
  });

  test("announces the skip link first", async ({ page, nvda }) => {
    await page.goto("/docs/authorities/");
    await nvda.navigateToWebContent();
    await nvda.next();
    const spoken = (await nvda.lastSpokenPhrase()).toLowerCase();
    await nvda.stop();
    expect(spoken).toContain("skip to content");
  });
});
