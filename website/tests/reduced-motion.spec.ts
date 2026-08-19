import { test, expect } from "@playwright/test";
import { ROUTES } from "./pages";

// Enforcement, not a lint: under prefers-reduced-motion: reduce, no animation may
// run above the ~instant threshold. document.getAnimations() catches CSS
// animations, transitions, and Web Animations API in one call. The glacier aurora
// is a static gradient, not an animation, so nothing should be active on load; a
// rogue continuous animation that ignores the guard would fail here.
const THRESHOLD_MS = 0.02; // just above the 0.01ms the reduce block sets

for (const route of ROUTES) {
  test(`reduced-motion: nothing animates on ${route.name}`, async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(route.path);
    const running = await page.evaluate((thr) => {
      return document
        .getAnimations()
        .map((a) => {
          const timing = a.effect && "getTiming" in a.effect ? a.effect.getTiming() : null;
          const dur = timing && typeof timing.duration === "number" ? timing.duration : 0;
          const el = a.effect && "target" in a.effect ? (a.effect as KeyframeEffect).target : null;
          return { dur, node: el ? el.nodeName + "." + (el as HTMLElement).className : "?" };
        })
        .filter((x) => x.dur > thr);
    }, THRESHOLD_MS);
    expect(
      running,
      `animations above ${THRESHOLD_MS}ms under reduce: ${JSON.stringify(running)}`,
    ).toEqual([]);
  });
}

test("reduced-motion: transitions collapse to ~instant", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/authorities/");
  // The skip link slides in with a transform transition; under reduce the custom
  // CSS must collapse it to ~0.
  const dur = await page
    .locator(".sl-skip-link")
    .evaluate((el) => getComputedStyle(el).transitionDuration);
  // 0.01ms == 0.00001s
  expect(parseFloat(dur)).toBeLessThanOrEqual(0.001);
});
