# Asset build rules

- Pin CSS framework and build tool versions in `package.json` and retain
  `package-lock.json` for reproducibility.
- Read upstream assets from installed packages; preserve licenses and remove
  source map references when not shipping source maps.
- Keep CSS transformations deterministic and syntax-aware. Verify selectors
  containing pseudo-classes, attributes, and nested framework rules.
- Regenerate assets with `npm ci --ignore-scripts` and `npm run build:themes`.
  Go consumers must not need Node.js to build shipped adapters.
