import { expect, test } from '@playwright/test';

const basePort = Number(process.env.GOMVU_BROWSER_BASE_PORT ?? 18080);
const counterURL = `http://127.0.0.1:${basePort}`;
const usersURL = `http://127.0.0.1:${basePort + 1}`;
const themesURL = `http://127.0.0.1:${basePort + 2}`;
const componentsURL = `http://127.0.0.1:${basePort + 3}`;
const themes = ['default', 'bootstrap', 'bulma', 'pico'];

async function expectNoPageOverflow(page) {
  expect(await page.evaluate(() =>
    document.documentElement.scrollWidth <= window.innerWidth,
  )).toBe(true);
}

test('counter swaps without replacing the document and keeps layout', async ({ page }) => {
  await page.goto(counterURL);
  const landmark = await page.locator('main').elementHandle();
  await expect(page.locator('#counter output')).toHaveText('0');
  await page.getByRole('button', { name: 'Increment', exact: true }).click();
  await expect(page.locator('#counter output')).toHaveText('1');
  expect(await landmark.evaluate((element) => element.isConnected)).toBe(true);
  await expect(page.locator('#counter')).toHaveCount(1);
  await expect(page.locator('#counter > .stack')).toHaveCSS('display', 'flex');
  await expect(page.locator('#counter > .stack')).toHaveCSS('gap', '16px');
  await page.getByRole('button', { name: 'Decrement', exact: true }).click();
  await expect(page.locator('#counter output')).toHaveText('0');
  await page.getByRole('button', { name: 'Increment', exact: true }).click();
  await expect(page.locator('#counter output')).toHaveText('1');
  await page.getByRole('button', { name: 'Reset', exact: true }).click();
  await expect(page.locator('#counter output')).toHaveText('0');
  await expectNoPageOverflow(page);
});

test('counter plain forms work without JavaScript', async ({ browser }, testInfo) => {
  const context = await browser.newContext({
    javaScriptEnabled: false,
    viewport: testInfo.project.use.viewport,
    colorScheme: testInfo.project.use.colorScheme,
  });
  try {
    const page = await context.newPage();
    await page.goto(counterURL);
    await page.getByRole('button', { name: 'Increment', exact: true }).click();
    await expect(page.locator('#counter output')).toHaveText('1');
    await expect(page).toHaveURL(`${counterURL}/`);
    await page.reload();
    await expect(page.locator('#counter output')).toHaveText('1');
  } finally {
    await context.close();
  }
});

test('pagination swaps results, pushes URL, and styles reusable cards', async ({ page }) => {
  await page.goto(`${usersURL}/users?page=1`);
  const landmark = await page.locator('main').elementHandle();
  await page.getByRole('link', { name: 'Next', exact: true }).click();
  await expect(page.locator('#users-results [aria-current="page"]')).toHaveText('2');
  await expect(page).toHaveURL(`${usersURL}/users?page=2`);
  expect(await landmark.evaluate((element) => element.isConnected)).toBe(true);
  await expect(page.locator('#users-results')).toHaveCount(1);
  await expect(page.locator('#users-results > .stack')).toHaveCSS('display', 'flex');
  await expect(page.locator('th[scope="row"]').first()).toHaveText('Margaret Lovelace');
  await page.reload();
  await expect(page.locator('#users-results [aria-current="page"]')).toHaveText('2');
  await page.getByRole('link', { name: 'View reusable user cards' }).click();
  const cards = page.locator('.user-card');
  await expect(cards).toHaveCount(6);
  await expect(cards.first()).toHaveCSS('border-top-width', '1px');
  await expect(cards.first().locator('.card__body')).toHaveCSS('padding', '16px');
  await expect(cards.first().locator('.stack')).toHaveCSS('display', 'flex');
  await expectNoPageOverflow(page);
});

for (const theme of themes) {
  test(`${theme}: computed styles, theme form, native dialog, plain table`, async ({ page }, testInfo) => {
    const mode = testInfo.project.use.colorScheme;
    await page.goto(`${componentsURL}/?theme=${theme}&mode=${mode}&tab=activity`);
    await expect(page.locator('html')).toHaveAttribute('data-theme-name', theme);
    const grid = page.locator('.grid--columns-2');
    await expect(grid).toHaveCSS('display', 'grid');
    const columns = await grid.evaluate((element) =>
      getComputedStyle(element).gridTemplateColumns.split(' ').length,
    );
    expect(columns).toBe(testInfo.project.use.viewport.width === 390 ? 1 : 2);
    const metrics = await page.locator('button.button--primary').first().evaluate((button) => ({
      height: button.getBoundingClientRect().height,
      background: getComputedStyle(button).backgroundColor,
    }));
    expect(metrics.height).toBeGreaterThanOrEqual(24);
    expect(metrics.background).not.toBe('rgba(0, 0, 0, 0)');
    const padding = await page.locator('.card__body').first().evaluate((body) =>
      parseFloat(getComputedStyle(body).paddingLeft) +
      parseFloat(getComputedStyle(body.closest('article')).paddingLeft),
    );
    expect(padding).toBeGreaterThan(0);
    await expect(page.locator('input[name="tab"]')).toHaveCSS('display', 'none');
    await expectNoPageOverflow(page);
    await page.getByRole('button', { name: 'Hover or focus', exact: true }).focus();
    await expect(page.getByRole('tooltip')).toBeVisible();
    await expectNoPageOverflow(page);

    const nextTheme = themes[(themes.indexOf(theme) + 1) % themes.length];
    const nextMode = mode === 'dark' ? 'light' : 'dark';
    await page.getByLabel('CSS framework').selectOption(nextTheme);
    await page.getByLabel('Mode', { exact: true }).selectOption(nextMode);
    await page.getByRole('button', { name: 'Apply theme' }).click();
    await expect(page).toHaveURL(
      `${componentsURL}/?theme=${nextTheme}&mode=${nextMode}&tab=activity`,
    );
    await expect(page.locator('html')).toHaveAttribute('data-theme-name', nextTheme);
    await expect(page.locator('html')).toHaveAttribute('data-theme', nextMode);
    await expect(page.locator('#gallery-tabs-panel')).toContainText('Three deployments');

    await page.getByRole('button', { name: 'Open dialog', exact: true }).click();
    await expect(page.locator('#invite-dialog')).toBeVisible();
    expect(await page.locator('#invite-dialog').evaluate((el) => el.matches(':popover-open')))
      .toBe(true);
    await page.getByRole('button', { name: 'Close', exact: true }).click();
    await expect(page.locator('#invite-dialog')).toBeHidden();

    await page.goto(`${themesURL}/?theme=${theme}&mode=${mode}`);
    await expect(page.locator('table')).toHaveCSS('display', 'table');
    const cellPadding = await page.locator('td').first().evaluate((el) =>
      parseFloat(getComputedStyle(el).paddingLeft),
    );
    expect(cellPadding).toBeGreaterThan(0);
    await expect(page.locator('main')).toHaveCount(1);
    await expectNoPageOverflow(page);
  });
}
