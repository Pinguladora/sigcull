import { test } from "@playwright/test";
import { VISUAL_ROUTES, THEMES } from "./pages";
import { visualCheck } from "./visual-helper";

// Screenshot each representative page in light and dark, diffed against a
// committed baseline with odiff. Catches theme/contrast regressions (the neon
// glacier background, the dark search modal) that a rule engine will not flag.
for (const theme of THEMES) {
  test.describe(`visual ${theme}`, () => {
    for (const route of VISUAL_ROUTES) {
      test(route.name, async ({ page }) => {
        await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
        await page.goto(route.path);
        await visualCheck(page, `${route.name}-${theme}`);
      });
    }
  });
}
