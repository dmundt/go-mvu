import { expect, test } from '@playwright/test';

test('newer pagination response wins after slower response is released', async ({ page }) => {
  const base = `http://127.0.0.1:${Number(process.env.GOMVU_BROWSER_BASE_PORT ?? 18080) + 1}`;
  let releaseOld;
  const released = new Promise(resolve => { releaseOld = resolve; });
  let signalReady;
  const ready = new Promise(resolve => { signalReady = resolve; });
  let signalSettled;
  const settled = new Promise(resolve => { signalSettled = resolve; });
  let failure;
  let oldRequest;
  await page.route(`${base}/users?page=2`, async route => {
    oldRequest = route.request();
    try {
      const response = await route.fetch();
      signalReady();
      await released;
      await route.fulfill({ response });
    } catch (error) {
      failure = error;
      signalReady();
    } finally {
      signalSettled();
    }
  });
  try {
    await page.goto(`${base}/users?page=1`);
    await expect(page.locator('.pagination')).toHaveAttribute('hx-sync', 'closest main:replace');
    await expect(page.locator('.pagination')).not.toHaveCSS('display', 'none');
    await page.locator('a[href="/users?page=2"]').first().click();
    await ready;
    if (failure) throw failure;
    await page.locator('a[href="/users?page=9"]').first().click();
    await expect(page.locator('.pagination [aria-current="page"]')).toHaveText('9');
    releaseOld();
    await settled;
    if (failure) {
      // The previous browser request is intentionally canceled by hx-sync.
      // Do not suppress unrelated interception or fixture errors.
      expect(oldRequest.failure()?.errorText, failure.message).toBe('net::ERR_ABORTED');
    }
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.pagination [aria-current="page"]')).toHaveText('9');
    await expect(page).toHaveURL(`${base}/users?page=9`);
    await page.reload();
    await expect(page.locator('.pagination [aria-current="page"]')).toHaveText('9');
  } finally {
    releaseOld();
  }
});
