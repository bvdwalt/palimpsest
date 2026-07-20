import { test, expect } from "@playwright/test";

test("pages list loads without a 401 even when /api/config is slow", async ({ page }) => {
  await page.route("**/api/config", async (route) => {
    await new Promise((r) => setTimeout(r, 500));
    await route.continue();
  });

  const pagesStatuses: number[] = [];
  page.on("response", (res) => {
    if (new URL(res.url()).pathname === "/api/pages") pagesStatuses.push(res.status());
  });

  await page.goto("/");
  await page.waitForLoadState("networkidle");

  expect(pagesStatuses).not.toContain(401);
});
