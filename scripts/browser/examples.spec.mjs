import { expect, test } from '@playwright/test';

const basePort = Number(process.env.GOMVU_BROWSER_BASE_PORT ?? 18080);
const helloURL = `http://127.0.0.1:${basePort + 5}`;
const adminURL = `http://127.0.0.1:${basePort + 6}`;

test('hello uses a plain, styled document without framework runtime or scripts', async ({ page }) => {
  await page.goto(helloURL);
  await expect(page).toHaveTitle('Hello World!');
  await expect(page.getByRole('heading', { name: 'Hello World!' })).toBeVisible();
  const size = await page.getByRole('heading').evaluate((element) =>
    parseFloat(getComputedStyle(element).fontSize),
  );
  expect(size).toBeGreaterThan(20);
  await expect(page.locator('script')).toHaveCount(0);
});

test('admin filters, pages, edits and refreshes independent regions', async ({ page }) => {
  await page.goto(`${adminURL}/admin`);
  await expect(page.getByRole('table', { name: 'Members' })).toBeVisible();
  await expect(page.locator('th[scope="row"]').first()).toHaveCSS('border-bottom-style', 'solid');
  await expect(page.locator('#admin-overview .grid')).toHaveCSS('display', 'grid');
  await page.getByRole('link', { name: 'Next', exact: true }).click();
  await expect(page.locator('th[scope="row"]').first()).toHaveText('Donald Example 06');
  await expect(page).toHaveURL(/page=2/);
  await page.goBack();
  await expect(page.locator('th[scope="row"]').first()).toHaveText('Ada Example 01');
  await page.goForward();
  await expect(page.locator('th[scope="row"]').first()).toHaveText('Donald Example 06');
  const overview = await page.locator('#admin-overview').elementHandle();
  const main = await page.locator('main').elementHandle();
  await page.getByLabel('Search members').fill('Example');
  await page.getByLabel('Filter by role').selectOption('Admin');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByText('6 matching members', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Next', exact: true }).click();
  await expect(page.getByRole('rowheader')).toHaveText(['Margaret Example 16']);
  expect(new URL(page.url()).searchParams.get('q')).toBe('Example');
  expect(new URL(page.url()).searchParams.get('role')).toBe('Admin');
  expect(new URL(page.url()).searchParams.get('page')).toBe('2');
  await page.getByLabel('Search members').fill('Ada');
  await page.getByLabel('Filter by role').selectOption('Admin');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByText('3 matching members', { exact: true })).toBeVisible();
  expect(await overview.evaluate((element) => element.isConnected)).toBe(true);

  const opener = page.getByRole('button', { name: 'Quick edit Ada Example 01', exact: true });
  await opener.focus();
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Quick edit member' });
  await expect(dialog).toBeVisible();
  await expect(dialog).not.toHaveAttribute('aria-modal', 'true');
  expect(await dialog.evaluate(element => element.matches(':popover-open'))).toBe(true);
  await expect(dialog).toHaveCSS('position', 'fixed');
  await expect(page.getByLabel('Member name')).toHaveValue('Ada Example 01');
  await expect(page.getByLabel('Member name')).toBeFocused();
  await expect(page.getByLabel('Member name')).toHaveCSS('border-top-style', 'solid');
  await page.getByLabel('Member name').fill('A');
  const validation = page.waitForResponse(response =>
    response.url().endsWith('/admin/save') && response.status() === 422);
  await page.getByRole('button', { name: 'Save member', exact: true }).click();
  await validation;
  await expect(dialog).toBeVisible();
  await expect(page.getByText('Use 2 to 80 characters.', { exact: true })).toBeVisible();
  await expect(page.getByLabel('Member name')).toHaveValue('A');
  await page.getByLabel('Member name').fill('Ada Updated');
  await page.getByLabel('Member role').selectOption('Viewer');
  await page.getByLabel('Member name').press('Enter');
  await expect(page.getByText('Member saved.', { exact: false })).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole('button', { name: 'Apply filters', exact: true })).toBeFocused();
  await expect(page.getByRole('rowheader', { name: 'Ada Example 01', exact: true })).toHaveCount(0);
  await expect(page.locator('#save-status')).toHaveAttribute('role', 'status');
  await expect(page.locator('#save-status')).toHaveCount(1);
  expect(await overview.evaluate((element) => element.isConnected)).toBe(true);
  const workspace = await page.locator('#admin-workspace').elementHandle();
  await page.getByRole('button', { name: 'Refresh overview', exact: true }).click();
  await expect(page.locator('#admin-overview .stat').filter({ hasText: 'Saved changes' })
    .locator('.stat__value')).toHaveText('1');
  expect(await workspace.evaluate((element) => element.isConnected)).toBe(true);
  expect(await main.evaluate((element) => element.isConnected)).toBe(true);
  await expect(page.locator('#admin-workspace')).toHaveCount(1);
  await expect(page.locator('#admin-overview')).toHaveCount(1);
  expect(await page.evaluate(() =>
    document.documentElement.scrollWidth <= window.innerWidth,
  )).toBe(true);
  await page.getByLabel('Search members').fill('No such member');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByText('No matching members', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Dark', exact: true }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(page.getByText('No matching members', { exact: true })).toBeVisible();
});

test('quick editor supports native dismissal, repeated open and non-modal background access', async ({ page }) => {
  await page.goto(`${adminURL}/admin`);
  const opener = page.getByRole('button', { name: 'Quick edit Ada Example 01', exact: true });
  const dialog = page.getByRole('dialog', { name: 'Quick edit member' });
  await opener.click();
  await expect(page.getByLabel('Member name')).toHaveValue('Ada Example 01');
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();
  await opener.click();
  await expect(dialog).toBeVisible();
  await expect(page.getByLabel('Member name')).toHaveValue('Ada Example 01');
  await page.getByLabel('Member name').fill('Unsubmitted draft');
  await page.getByRole('button', { name: 'Cancel quick edit', exact: true }).click();
  await expect(dialog).toBeHidden();
  await opener.click();
  await expect(page.getByLabel('Member name')).toHaveValue('Ada Example 01');
  await page.getByRole('button', { name: 'Refresh overview', exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.locator('#admin-overview .stat').filter({ hasText: 'Saved changes' })
    .locator('.stat__value')).toHaveText('0');
});

async function holdEdit(page, id) {
  let release;
  const released = new Promise(resolve => { release = resolve; });
  let ready;
  const responseReady = new Promise(resolve => { ready = resolve; });
  let done;
  const finished = new Promise(resolve => { done = resolve; });
  let failure;
  let request;
  let canceled;
  const cancellation = new Promise(resolve => { canceled = resolve; });
  page.on('requestfailed', failed => {
    if (failed === request) canceled(failed.failure()?.errorText);
  });
  await page.route(`${adminURL}/admin/edit?id=${id}`, async route => {
    request = route.request();
    try {
      const response = await route.fetch();
      ready();
      await released;
      await route.fulfill({ response });
    } catch (error) {
      failure = error;
      ready();
    } finally {
      done();
    }
  });
  return {
    ready: responseReady, release, cancellation,
    async finish() {
      await finished;
      if (failure) expect(request.failure()?.errorText, failure.message).toBe('net::ERR_ABORTED');
    },
  };
}

for (const next of ['another member', 'cancel']) {
  test(`slow editor cannot override ${next}`, async ({ page }) => {
    const held = await holdEdit(page, 1);
    try {
      await page.goto(`${adminURL}/admin`);
      await page.getByRole('button', { name: 'Quick edit Ada Example 01', exact: true }).click();
      await held.ready;
      if (next === 'cancel') {
        await page.getByRole('button', { name: 'Cancel quick edit', exact: true }).click();
        await expect(page.getByRole('dialog')).toBeHidden();
      } else {
        await page.getByRole('button', { name: 'Quick edit Grace Example 02', exact: true }).click();
        await expect(page.getByLabel('Member name')).toHaveValue('Grace Example 02');
      }
      expect(await held.cancellation).toBe('net::ERR_ABORTED');
      held.release();
      await held.finish();
      if (next === 'cancel') {
        await expect(page.getByRole('dialog')).toBeHidden();
        await page.getByRole('button', { name: 'Quick edit Grace Example 02', exact: true }).click();
      }
      await expect(page.getByLabel('Member name')).toHaveValue('Grace Example 02');
    } finally {
      held.release();
    }
  });
}

test('Escape during load never reopens native popover when response arrives', async ({ page }) => {
  const held = await holdEdit(page, 1);
  try {
    await page.goto(`${adminURL}/admin`);
    await page.getByRole('button', { name: 'Quick edit Ada Example 01', exact: true }).click();
    await held.ready;
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toBeHidden();
    held.release();
    await held.finish();
    await expect(page.locator('#member-editor-content input[name="name"]'))
      .toHaveValue('Ada Example 01');
    await expect(page.getByRole('dialog')).toBeHidden();
  } finally {
    held.release();
  }
});

test('admin works without JavaScript and keeps validation through redirects', async ({ browser }, testInfo) => {
  const context = await browser.newContext({
    javaScriptEnabled: false,
    viewport: testInfo.project.use.viewport,
    colorScheme: testInfo.project.use.colorScheme,
  });
  try {
    const page = await context.newPage();
    await page.goto(`${adminURL}/admin`);
    await page.getByLabel('Search members').fill('Example');
    await page.getByLabel('Filter by role').selectOption('Admin');
    await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
    await page.getByRole('link', { name: 'Next', exact: true }).click();
    await expect(page.getByRole('rowheader')).toHaveText(['Margaret Example 16']);
    expect(new URL(page.url()).searchParams.get('q')).toBe('Example');
    expect(new URL(page.url()).searchParams.get('role')).toBe('Admin');
    expect(new URL(page.url()).searchParams.get('page')).toBe('2');
    await page.getByLabel('Filter by role').selectOption('');
    await page.getByLabel('Search members').fill('Grace');
    await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
    await expect(page.getByText('3 matching members', { exact: true })).toBeVisible();
    await page.getByRole('link', { name: 'Edit Grace Example 02', exact: true }).click();
    await page.getByLabel('Member name').fill('G');
    await page.getByRole('button', { name: 'Save member', exact: true }).click();
    await expect(page).toHaveURL(`${adminURL}/admin/view`);
    await expect(page.getByText('Use 2 to 80 characters.', { exact: true })).toBeVisible();
    await expect(page.getByLabel('Member name')).toHaveValue('G');
    await page.getByLabel('Member name').fill('Grace Updated');
    await page.getByRole('button', { name: 'Save member', exact: true }).click();
    await expect(page.getByRole('rowheader', { name: 'Grace Updated' })).toBeVisible();
    await page.reload();
    await expect(page.getByRole('rowheader', { name: 'Grace Updated' })).toBeVisible();
    await page.getByRole('link', { name: 'Edit Grace Updated', exact: true }).click();
    await page.getByLabel('Member name').fill('Unsaved name');
    await page.getByRole('link', { name: 'Cancel editing', exact: true }).click();
    await expect(page.getByRole('rowheader', { name: 'Grace Updated' })).toBeVisible();
    await expect(page.getByLabel('Member name')).toHaveCount(0);
  } finally {
    await context.close();
  }
});
