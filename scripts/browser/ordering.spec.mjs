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
  let signalCanceled;
  const canceled = new Promise(resolve => { signalCanceled = resolve; });
  page.on('requestfailed', request => {
    if (request === oldRequest) signalCanceled(request.failure()?.errorText);
  });
  const swaps = [];
  await page.exposeFunction('recordOrderingSwap', detail => swaps.push(detail));
  await page.addInitScript(() => {
    document.addEventListener('htmx:afterSettle', event => {
      window.recordOrderingSwap({
        page: document.querySelector('.pagination [aria-current="page"]')?.textContent,
        path: event.detail.pathInfo?.requestPath,
      });
    });
  });
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
    expect(await canceled).toBe('net::ERR_ABORTED');
    releaseOld();
    await settled;
    if (failure) {
      // The previous browser request is intentionally canceled by hx-sync.
      // Do not suppress unrelated interception or fixture errors.
      expect(oldRequest.failure()?.errorText, failure.message).toBe('net::ERR_ABORTED');
    }
    await expect.poll(() => swaps.some(swap => swap.path === '/users?page=9' && swap.page === '9')).toBe(true);
    expect(swaps.some(swap => swap.path === '/users?page=2')).toBe(false);
    await expect(page.locator('.pagination [aria-current="page"]')).toHaveText('9');
    await expect(page).toHaveURL(`${base}/users?page=9`);
    await page.reload();
    await expect(page.locator('.pagination [aria-current="page"]')).toHaveText('9');
  } finally {
    releaseOld();
  }
});
