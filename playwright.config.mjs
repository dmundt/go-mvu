import { defineConfig } from '@playwright/test';

const examples = ['counter', 'users', 'themes', 'components', 'contact'];
const basePort = Number(process.env.GOMVU_BROWSER_BASE_PORT ?? 18080);
if (!Number.isInteger(basePort) || basePort < 1024 || basePort > 65531) {
  throw new Error('GOMVU_BROWSER_BASE_PORT must be an integer from 1024 to 65531');
}

export default defineConfig({
  testDir: './scripts/browser',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 2,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    browserName: 'chromium',
    channel: process.env.PLAYWRIGHT_CHANNEL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: ['desktop', 'mobile'].flatMap((size) =>
    ['light', 'dark'].map((colorScheme) => ({
      name: `${size}-${colorScheme}`,
      use: {
        colorScheme,
        viewport: { width: size === 'desktop' ? 1280 : 390, height: 900 },
      },
    })),
  ),
  webServer: examples.map((example, index) => {
    const readyPath = example === 'users' ? '/users' : example === 'contact' ? '/contact' : '/';
    return {
      command: `go test -count=1 -run "^TestBrowserServer$" -v -timeout=0 ./examples/${example}`,
      url: `http://127.0.0.1:${basePort + index}${readyPath}`,
      env: { GOMVU_BROWSER_ADDR: `127.0.0.1:${basePort + index}` },
      timeout: 180_000,
      reuseExistingServer: false,
      gracefulShutdown: { signal: 'SIGTERM', timeout: 5_000 },
    };
  }),
});
