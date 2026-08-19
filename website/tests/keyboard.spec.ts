import { test, expect } from "@playwright/test";

// WCAG 2.2 interaction checks for the two things a keyboard user reaches first on
// a Starlight page: the skip link and the built-in search dialog. Both must be
// operable and escapable by keyboard (2.1.1, 2.1.2, 2.4.1).

test.describe("skip link", () => {
  test("is the first focusable element and targets the content", async ({ page }) => {
    await page.goto("/authorities/");
    await page.keyboard.press("Tab");
    const skip = page.locator(".sl-skip-link");
    await expect(skip).toBeFocused();
    await expect(skip).toHaveAttribute("href", "#_top");
  });
});

test.describe("search dialog", () => {
  test("opens with Ctrl+K, focuses the input, closes on Escape", async ({ page }) => {
    await page.goto("/authorities/");
    const dialog = page.locator("dialog[aria-label='Search']");
    await expect(dialog).toBeHidden();

    await page.keyboard.press("Control+k");
    await expect(dialog).toBeVisible();

    // Pagefind lazy-loads its input; what matters for WCAG is that focus moves
    // into the dialog (Starlight may focus the input or its container).
    const input = dialog.locator("input").first();
    await expect(input).toBeVisible({ timeout: 10_000 });
    await expect.poll(() => dialog.evaluate((d) => d.contains(document.activeElement))).toBe(true);

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });
});
