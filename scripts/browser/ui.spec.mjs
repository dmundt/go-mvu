import { expect, test } from '@playwright/test';

const basePort = Number(process.env.GOMVU_BROWSER_BASE_PORT ?? 18080);
const counterURL = `http://127.0.0.1:${basePort}`;
const usersURL = `http://127.0.0.1:${basePort + 1}`;
const themesURL = `http://127.0.0.1:${basePort + 2}`;
const componentsURL = `http://127.0.0.1:${basePort + 3}`;
const contactURL = `http://127.0.0.1:${basePort + 4}`;
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
  await expect(page.locator('#counter > .stack')).toHaveCSS('gap', '20px');
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

test('contact keyboard submit swaps, preserves values, and styles fields', async ({ page }) => {
  await page.goto(`${contactURL}/contact`);
  const landmark = await page.locator('main').elementHandle();
  const name = page.getByLabel('Your name');
  await expect(name).toHaveCSS('border-top-style', 'solid');
  await expect(name).toHaveCSS('border-top-width', '1px');
  const inputWidth = await name.evaluate((element) => element.getBoundingClientRect().width);
  expect(inputWidth).toBeGreaterThan(await page.evaluate(() => window.innerWidth * 0.4));
  expect(await name.evaluate((element) => parseFloat(getComputedStyle(element).paddingLeft)))
    .toBeGreaterThan(0);

  await name.fill('A');
  await page.getByLabel('Email address').fill('ada@example.test');
  await page.getByLabel('Company (optional)').fill('Analytical Engines');
  await page.getByLabel('Topic').selectOption('support');
  await page.getByLabel('How can we help?').fill('Please help with this request.');
  await page.getByLabel('You may reply to my request.').check();
  const invalidSubmission = page.waitForRequest((request) =>
    request.method() === 'POST' && request.url() === `${contactURL}/contact`,
  );
  await name.press('Enter');
  expect((await invalidSubmission).headers()['hx-request']).toBe('true');
  await expect(page.locator('#name-error')).toHaveText('Enter at least two characters.');
  expect(await landmark.evaluate((element) => element.isConnected)).toBe(true);
  await expect(page.locator('#contact-form')).toHaveCount(1);
  await expect(page.getByLabel('Your name')).toHaveValue('A');
  await expect(page.getByLabel('Your name')).toHaveAttribute('aria-invalid', 'true');
  await expect(page.getByLabel('Company (optional)')).toHaveValue('Analytical Engines');
  await expect(page.getByLabel('Topic')).toHaveValue('support');
  await expect(page.getByLabel('How can we help?')).toHaveValue('Please help with this request.');
  await expect(page.getByLabel('You may reply to my request.')).toBeChecked();

  await page.getByLabel('Your name').fill('Ada Lovelace');
  await page.getByLabel('Email address').press('Enter');
  await expect(page.locator('#contact-form')).toContainText('Request received');
  expect(await landmark.evaluate((element) => element.isConnected)).toBe(true);
  await expectNoPageOverflow(page);
});

test('contact keyboard submit uses post-redirect-get without JavaScript', async ({ browser }, testInfo) => {
  const context = await browser.newContext({
    javaScriptEnabled: false,
    viewport: testInfo.project.use.viewport,
    colorScheme: testInfo.project.use.colorScheme,
  });
  try {
    const page = await context.newPage();
    await page.goto(`${contactURL}/contact`);
    await page.getByLabel('Your name').fill('A');
    await page.getByLabel('Email address').fill('ada@example.test');
    await page.getByLabel('Topic').selectOption('support');
    await page.getByLabel('How can we help?').fill('Please help with this request.');
    await page.getByLabel('You may reply to my request.').check();
    await page.getByLabel('Your name').press('Enter');
    await expect(page).toHaveURL(`${contactURL}/contact`);
    await expect(page.locator('#name-error')).toHaveText('Enter at least two characters.');
    await expect(page.getByLabel('Your name')).toHaveValue('A');

    await page.getByLabel('Your name').fill('Ada Lovelace');
    await page.getByLabel('Email address').press('Enter');
    await expect(page.locator('#contact-form')).toContainText('Request received');
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
  await page.goBack();
  await expect(page).toHaveURL(`${usersURL}/users?page=1`);
  await expect(page.locator('#users-results [aria-current="page"]')).toHaveText('1');
  await expect(page.locator('th[scope="row"]').first()).toHaveText('Ada Lovelace');
  await page.goForward();
  await expect(page).toHaveURL(`${usersURL}/users?page=2`);
  await expect(page.locator('#users-results [aria-current="page"]')).toHaveText('2');
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

test('pagination works without JavaScript', async ({ browser }, testInfo) => {
  const context = await browser.newContext({
    javaScriptEnabled: false,
    viewport: testInfo.project.use.viewport,
    colorScheme: testInfo.project.use.colorScheme,
  });
  try {
    const page = await context.newPage();
    await page.goto(`${usersURL}/users?page=1`);
    await page.getByRole('link', { name: 'Next', exact: true }).click();
    await expect(page).toHaveURL(`${usersURL}/users?page=2`);
    await expect(page.locator('th[scope="row"]').first()).toHaveText('Margaret Lovelace');
    await page.reload();
    await expect(page.locator('#users-results [aria-current="page"]')).toHaveText('2');
    await page.getByRole('link', { name: 'Previous', exact: true }).click();
    await expect(page).toHaveURL(`${usersURL}/users?page=1`);
    await expect(page.locator('th[scope="row"]').first()).toHaveText('Ada Lovelace');
  } finally {
    await context.close();
  }
});

async function expectVisibleFocus(locator) {
  await expect(locator).toBeFocused();
  await expect.poll(() => locator.evaluate((el) => {
    const style = getComputedStyle(el);
    return el.matches(':focus-visible') &&
      ((style.outlineStyle !== 'none' && parseFloat(style.outlineWidth) > 0 &&
        style.outlineColor !== 'transparent' && style.outlineColor !== 'rgba(0, 0, 0, 0)') ||
        (style.boxShadow !== 'none' &&
          !style.boxShadow.startsWith('rgba(0, 0, 0, 0)')));
  })).toBe(true);
}

for (const theme of themes) {
  test(`${theme}: keyboard navigation, activation, dismissal, and focus`, async ({ page }, testInfo) => {
    await page.goto(`${componentsURL}/?theme=${theme}&mode=${testInfo.project.use.colorScheme}`);
    const framework = page.getByRole('combobox', { name: 'CSS framework', exact: true });
    await framework.focus();
    await page.keyboard.press('Tab');
    await expectVisibleFocus(page.getByRole('combobox', { name: 'Mode', exact: true }));
    await page.keyboard.press('Tab');
    await expectVisibleFocus(page.getByRole('button', { name: 'Apply theme', exact: true }));

    const trigger = page.getByRole('button', { name: 'Open dialog', exact: true });
    await trigger.focus();
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Tab');
    await expectVisibleFocus(trigger);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Invite teammate', exact: true });
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Tab');
    const close = dialog.getByRole('button', { name: 'Close', exact: true });
    await expectVisibleFocus(close);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
    await page.keyboard.press('Space');
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Tab');
    await expect(close).toBeFocused();
    await page.keyboard.press('Space');
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();

    const tooltip = page.getByRole('button', { name: 'Show help', exact: true });
    await tooltip.focus();
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Tab');
    await expectVisibleFocus(tooltip);
    await expect(tooltip).toHaveAccessibleDescription('Tooltips describe their trigger.');
    await page.keyboard.press('Space');
    await expect(page.getByRole('tooltip')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('tooltip')).toBeHidden();
    await expect(tooltip).toBeFocused();
  });

  test(`${theme}: system palette follows OS changes and reduces motion`, async ({ page }) => {
    const palette = () => page.locator('body').evaluate((el) => {
      const style = getComputedStyle(el);
      return { background: style.backgroundColor, color: style.color };
    });
    const explicit = {};
    for (const mode of ['light', 'dark']) {
      await page.goto(`${componentsURL}/?theme=${theme}&mode=${mode}`);
      explicit[mode] = await palette();
    }
    expect(explicit.light).not.toEqual(explicit.dark);
    await page.emulateMedia({ colorScheme: 'light', reducedMotion: 'no-preference' });
    await page.goto(`${componentsURL}/?theme=${theme}&mode=system`);
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'system');
    await expect.poll(palette).toEqual(explicit.light);
    await page.emulateMedia({ colorScheme: 'dark' });
    await expect.poll(palette).toEqual(explicit.dark);
    await page.emulateMedia({ colorScheme: 'light' });
    await expect.poll(palette).toEqual(explicit.light);

    const duration = (selector, property) => page.locator(selector).first().evaluate(
      (el, name) => Math.max(...getComputedStyle(el)[name].split(',').map(parseFloat)),
      property,
    );
    expect(await duration('.tip__content', 'transitionDuration')).toBeLessThanOrEqual(0.00001);
    expect(await duration('.spinner__icon', 'animationDuration')).toBeGreaterThan(0.01);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await expect.poll(() => duration('.tip__content', 'transitionDuration')).toBeLessThanOrEqual(0.00001);
    await expect.poll(() => duration('.spinner__icon', 'animationDuration')).toBeLessThanOrEqual(0.001);
    await expect(page.locator('.spinner__icon').first()).toHaveCSS('animation-iteration-count', '1');
  });

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
    const formLayout = page.locator('.form-fields');
    await expect(formLayout).toHaveCSS('display', 'flex');
    await expect(formLayout).toHaveCSS('align-items', 'stretch');
    const fieldWidths = await page.getByLabel('Message', { exact: true }).evaluate((element) => ({
      control: element.getBoundingClientRect().width,
      layout: element.closest('.form-fields').getBoundingClientRect().width,
    }));
    expect(Math.abs(fieldWidths.control - fieldWidths.layout)).toBeLessThanOrEqual(1);
    await expect(page.locator('input[name="tab"]')).toHaveCSS('display', 'none');
    await expectNoPageOverflow(page);
    await page.getByRole('button', { name: 'Show help', exact: true }).click();
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
