import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { ROUTES, THEMES } from "./pages";

// axe-core sweep across every page template, in light and dark. Starlight keys
// dark mode off an inline script that reads prefers-color-scheme on load, so
// emulateMedia exercises both the glacier aurora and the search dialog overrides.
// Tags cover WCAG 2.0-2.2 A/AA.
const WCAG_TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

for (const theme of THEMES) {
  for (const route of ROUTES) {
    test(`a11y: ${route.name} [${theme}]`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
      await page.goto(route.path);
      const { violations } = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
      const summary = violations
        .map((v) => `  [${v.impact}] ${v.id} ×${v.nodes.length} — ${v.help}\n    ${v.helpUrl}`)
        .join("\n");
      expect(violations, violations.length ? `\naxe violations:\n${summary}` : undefined).toEqual(
        [],
      );
    });
  }
}
