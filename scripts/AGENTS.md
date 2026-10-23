# Asset build rules

- Pin CSS token-source and build tool versions in `package.json` and retain
  `package-lock.json` for reproducibility.
- Read Open Props assets from installed packages and preserve its license.
- Regenerate assets with `npm ci --ignore-scripts` and `npm run build:themes`.
  Go consumers must not need Node.js to use the embedded default theme.
- Browser tests live in `scripts/browser/`; Playwright config owns server
  lifecycle and viewport/mode projects. Test real computed styles and behavior,
  not selector presence alone; no arbitrary sleeps or retries hiding failures.
- Browser JavaScript is test-only; never ship it as application behavior.
