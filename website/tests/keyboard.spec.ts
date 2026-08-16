import { test, expect } from "@playwright/test";

// WCAG 2.2 interaction checks for the two custom widgets: the lazy search modal
// and the page-actions disclosure. Both must be operable and escapable by
// keyboard, and must return focus to their opener (2.1.1, 2.1.2, 2.4.3).

test.describe("skip link", () => {
  test("is the first focusable element and moves focus to the content", async ({ page }) => {
    await page.goto("/docs/authorities/");
    await page.keyboard.press("Tab");
    const skip = page.locator(".skip-link");
    await expect(skip).toBeFocused();
    await skip.press("Enter");
    await expect(page.locator("#main-content")).toBeFocused();
  });
});

test.describe("search modal", () => {
  test("opens with Ctrl+K, traps then returns focus, closes on Escape", async ({ page }) => {
    await page.goto("/docs/authorities/");
    const modal = page.locator("#pf-modal");
    await expect(modal).toBeHidden();

    await page.keyboard.press("Control+k");
    await expect(modal).toBeVisible();

    // Pagefind lazy-loads its input; focus must land inside the dialog.
    const input = modal.locator("input").first();
    await expect(input).toBeVisible({ timeout: 10_000 });
    await expect(input).toBeFocused();

    await page.keyboard.press("Escape");
    await expect(modal).toBeHidden();
  });
});

test.describe("page-actions disclosure", () => {
  test("toggles aria-expanded and closes on Escape", async ({ page }) => {
    await page.goto("/docs/authorities/");
    const trigger = page.locator(".pa-trigger");
    const menu = page.locator("#page-menu");

    await expect(trigger).toHaveAttribute("aria-expanded", "false");
    await expect(menu).toBeHidden();

    await trigger.click();
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
    await expect(menu).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(trigger).toHaveAttribute("aria-expanded", "false");
    await expect(menu).toBeHidden();
  });

  test("menu items are reachable by Tab and are real links/buttons", async ({ page }) => {
    await page.goto("/docs/authorities/");
    await page.locator(".pa-trigger").click();
    const items = page.locator("#page-menu .pa-item");
    await expect(items.first()).toBeVisible();
    // First item is the copy button, the rest are assistant links.
    await expect(items.first()).toHaveJSProperty("tagName", "BUTTON");
    expect(await items.count()).toBeGreaterThan(1);
  });
});
