import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import postcss from "postcss";

const root = new URL("../", import.meta.url);

const defaultFile = name => readFile(new URL(`theme/default/${name}`, root), "utf8");

test("adapter token requirements have one documented source", async () => {
  const guide = await readFile(new URL("docs/guides/themes.md", root), "utf8");
  assert.match(guide, /\]\(\.\.\/\.\.\/scripts\/theme-contract\.json\)/);
});

function contrastRatio(foreground, background = "#ffffff") {
  const luminance = hex => {
    const channels = hex.match(/[a-f\d]{2}/gi).map(value => parseInt(value, 16) / 255)
      .map(value => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4);
    return .2126 * channels[0] + .7152 * channels[1] + .0722 * channels[2];
  };
  const first = luminance(foreground);
  const second = luminance(background);
  return (Math.max(first, second) + .05) / (Math.min(first, second) + .05);
}

test("base CSS styles semantic tables, including headers and row states", async () => {
  const css = postcss.parse(await defaultFile("base.css"));
  const rules = new Map();
  css.walkRules(rule => rules.set(rule.selector.replace(/\s+/g, " "), rule));
  for (const selector of ["table", "caption", "th, td", "thead th, tfoot th, tfoot td"]) {
    assert.ok(rules.has(selector), `missing table selector ${selector}`);
  }
  const declarations = rule => Object.fromEntries(rule.nodes.map(node => [node.prop, node.value]));
  assert.equal(declarations(rules.get("table"))["border-collapse"], "collapse");
  assert.equal(declarations(rules.get("th, td"))["border-bottom"], "1px solid var(--border)");
  assert.equal(declarations(rules.get(":root"))["--surface-raised"], "var(--gray-0)");
  assert.equal(declarations(rules.get(":root"))["--muted"], "var(--gray-9)");
  assert.ok([...rules.keys()].some(selector => selector.includes('html[data-theme="dark"]') &&
    declarations(rules.get(selector))["--muted"] === "var(--gray-5)"), "dark palette needs muted text role");
  assert.ok([...rules.keys()].some(selector => selector.includes("tbody tr:focus-within")));
  assert.equal(declarations(rules.get(".stack")).gap, "var(--space-4)");
  assert.equal(declarations(rules.get(".grid")).gap, "var(--space-4)");
});

test("default design system stays scoped and includes both palettes and keyboard focus", async () => {
  const css = postcss.parse(await defaultFile("default.css"));
  let focused = false;
  let dark = false;
  let mobileShell = false;
  css.walkRules(rule => {
    for (const selector of rule.selectors) {
      assert.ok(selector.includes('html[data-theme-name="default"]'),
        `unscoped default refinement: ${selector}`);
    }
    if (rule.selector.includes(":focus-visible")) {
      focused ||= rule.nodes.some(node => node.prop === "outline" && /^(?:\d+px|var\(--focus-outline-width\)) solid/.test(node.value));
    }
    if (rule.selector.includes('[data-theme="dark"]')) dark = true;
  });
  css.walkAtRules("media", rule => {
    if (rule.params !== "(max-width: 48rem)") return;
    rule.walkRules(nested => {
      if (nested.selector.includes(".shell") &&
          nested.nodes.some(node => node.prop === "grid-template-columns" && node.value === "minmax(0, 1fr)")) {
        mobileShell = true;
      }
    });
  });
  assert.ok(focused, "missing visible keyboard focus");
  assert.ok(dark, "missing default dark palette");
  assert.ok(mobileShell, "mobile shell must collapse to one column");
});

test("default visual design uses Open Props indigo and neutral bordered tables", async () => {
  const css = postcss.parse(await defaultFile("default.css"));
  const rules = new Map();
  css.walkRules(rule => rules.set(rule.selector, Object.fromEntries(
    rule.nodes.filter(node => node.type === "decl").map(node => [node.prop, node.value]),
  )));
  const palette = rules.get('html[data-theme-name="default"]');
  const darkPalette = rules.get('html[data-theme-name="default"][data-theme="dark"]');
  const systemPalette = rules.get('html[data-theme-name="default"][data-theme="system"]');
  const links = rules.get(':where(html[data-theme-name="default"]) a');
  const cells = rules.get(':where(html[data-theme-name="default"]) :is(th, td)');
  const headers = rules.get(':where(html[data-theme-name="default"]) thead th');
  assert.equal(palette["--primary"], "var(--indigo-7)");
  assert.equal(palette["--success"], "var(--green-10)");
  assert.equal(palette["--warning"], "var(--yellow-11)");
  assert.equal(darkPalette["--primary"], "var(--indigo-3)");
  assert.equal(systemPalette["--primary"], "var(--indigo-3)");
  assert.notEqual(palette["--bg"], palette["--surface"]);
  assert.equal(palette["--surface-raised"], "var(--gray-0)");
  assert.equal(palette["--focus-ring"], "0 0 0 3px var(--primary)");
  assert.match(palette["--border"], /^color-mix\(/);
  assert.match(palette["--table-header"], /^color-mix\(/);
  assert.equal(links.color, "var(--primary)");
  assert.equal(links["text-decoration"], "none");
  assert.equal(cells.border, "1px solid var(--border)");
  assert.equal(headers.background, "var(--table-header)");
});

test("default light action and status colors meet WCAG AA text contrast", async () => {
  const subset = await defaultFile("open-props.css");
  const css = await defaultFile("default.css");
  const token = name => {
    const match = subset.match(new RegExp(`${name}:\\s*(#[0-9a-f]{6})`, "i"));
    assert.ok(match, `missing hex value for ${name}`);
    return match[1];
  };
  const semantic = [
    ["--primary", "--indigo-7"],
    ["--success", "--green-10"],
    ["--warning", "--yellow-11"],
    ["--danger", "--red-9"],
    ["--info", "--indigo-7"]
  ];
  for (const [role, prop] of semantic) {
    assert.match(css, new RegExp(`${role}:\\s*var\\(${prop}\\)`));
    assert.ok(contrastRatio(token(prop)) >= 4.5, `${role} must meet 4.5:1 on light surfaces`);
  }
});

test("default theme defines semantic type and Open Props spacing scales", async () => {
  const css = postcss.parse(await defaultFile("default.css"));
  const declarations = new Map();
  css.walkRules(rule => {
    if (rule.selector === 'html[data-theme-name="default"]') {
      for (const node of rule.nodes) {
        if (node.type === "decl") declarations.set(node.prop, node.value);
      }
    }
  });
  for (const [token, value] of Object.entries({
    "--font-family-sans": 'Inter, "Segoe UI", system-ui, sans-serif',
    "--font-size-xs": "var(--font-size-0)",
    "--font-size-sm": "calc(var(--font-size-md) * .875)",
    "--font-size-md": "var(--font-size-1)",
    "--font-size-lg": "calc(var(--font-size-md) * 1.125)",
    "--font-size-xl": "var(--font-size-3)",
    "--font-size-2xl": "var(--font-size-4)",
    "--font-size-body": "var(--font-size-md)",
    "--line-height-body": "var(--font-lineheight-3)",
    "--space-1": "var(--size-1)",
    "--space-2": "var(--size-2)",
    "--space-3": "var(--size-3)",
    "--space-4": "var(--size-4)",
    "--space-5": "var(--size-5)",
    "--space-6": "var(--size-6)",
    "--control-height": "2.5rem",
    "--content-max-width": "56.25rem",
    "--motion-fast": "120ms",
    "--motion-normal": "180ms",
    "--tracking-tight": "-.01em",
    "--tracking-label": ".04em",
    "--tracking-overline": ".06em",
    "--tracking-badge": ".02em"
  })) {
    assert.equal(declarations.get(token), value, `unexpected ${token}`);
  }
  assert.match(css.toString(), /font: var\(--font-weight-normal\) var\(--font-size-body\)\/var\(--line-height-body\) var\(--font-family-sans\)/);
});

test("shared widgets use semantic spacing, type, and restrained elevation tokens", async () => {
  const css = postcss.parse(await defaultFile("shared.css"));
  const rules = new Map();
  css.walkRules(rule => rules.set(rule.selector, Object.fromEntries(
    rule.nodes.filter(node => node.type === "decl").map(node => [node.prop, node.value]),
  )));
  assert.equal(rules.get("html")["--shared-shadow"], "var(--shadow-2, 0 4px 12px rgb(15 23 42 / 12%))");
  assert.equal(rules.get("html")["--shared-space-3"], "var(--space-3, 1rem)");
  assert.equal(rules.get("html")["--shared-space-4"], "var(--space-4, 1.25rem)");
  assert.equal(rules.get("html")["--shared-space-6"], "var(--space-6, 1.75rem)");
  assert.equal(rules.get("html")["--shared-line-height-tight"], "var(--line-height-tight, 1.25)");
  assert.equal(rules.get(".accordion__trigger").padding, "var(--shared-space-3) var(--shared-space-4)");
  assert.equal(rules.get(".dialog__body").padding, "var(--shared-space-3) var(--shared-space-5)");
  assert.equal(rules.get(".dialog")["box-shadow"], "var(--shared-shadow)");
  assert.equal(rules.get(".empty-state")["padding"], "var(--shared-space-6) var(--shared-space-5)");
  assert.equal(rules.get(".stat__value")["font-size"], "var(--shared-font-2xl)");
  assert.equal(rules.get(".dialog__close")["min-height"], "var(--shared-control-height)");
  assert.equal(rules.get(".tabset__tab")["min-height"], "var(--shared-control-height)");
  assert.equal(rules.get(".dropdown__menu a")["min-height"], "var(--shared-control-height)");
  const controlLabel = [...rules.entries()].find(([selector]) =>
    selector.includes(".switch") && selector.includes(".checkbox") && selector.includes(".radio"))?.[1];
  assert.equal(controlLabel?.["min-height"], "var(--shared-control-height)");
  assert.equal(rules.get(".dialog").background, "var(--surface-raised, var(--surface))");
  assert.equal(rules.get(".dropdown__menu").background, "var(--surface-raised, var(--surface))");
  assert.equal(rules.get(".tabset__tab[aria-current=\"page\"]").background, "var(--surface-raised, var(--surface))");
  assert.match(css.toString(), /@keyframes shared-pulse/);
  assert.doesNotMatch(css.toString(), /linear-gradient/);
  const reducedMotion = new Map();
  css.walkAtRules("media", rule => {
    if (rule.params !== "(prefers-reduced-motion: reduce)") return;
    rule.walkRules(nested => {
      reducedMotion.set(nested.selector, Object.fromEntries(
        nested.nodes.filter(node => node.type === "decl").map(node => [node.prop, node.value]),
      ));
    });

    test("shared tooltip popover is anchored and supports native dismissal", async () => {
      const css = postcss.parse(await defaultFile("shared.css"));
      const declarations = rule => Object.fromEntries(
        rule.nodes.filter(node => node.type === "decl").map(node => [node.prop, node.value]),
      );
      const rules = new Map();
      css.walkRules(rule => rules.set(rule.selector, declarations(rule)));
      assert.equal(rules.get(".tip")["anchor-scope"], "--tip");
      assert.equal(rules.get(".tip > button")["anchor-name"], "--tip");
      assert.equal(rules.get(".tip__content").margin, "auto");
      let anchoredPopover = false;
      css.walkAtRules("supports", atRule => {
        if (atRule.params !== "(position-area: block-end)") return;
        atRule.walkRules(rule => {
          if (rule.selector !== ".tip__content:popover-open") return;
          const values = declarations(rule);
          anchoredPopover = values["position-anchor"] === "--tip" &&
            values["position-area"] === "block-end" && values.margin === "0";
        });
      });
      assert.ok(anchoredPopover, "open tooltip popover should anchor to its trigger");
    });
  });
  assert.equal(reducedMotion.get(".skeleton__line")?.animation, "none");
  assert.equal(reducedMotion.get(".progress-field > progress:indeterminate")?.animation, "none");
  assert.equal(reducedMotion.get(".spinner__icon")?.animation, "none");
  assert.equal(rules.get(".switch > input.switch__input")["transition"],
    "background-color var(--shared-motion-normal) ease, border-color var(--shared-motion-normal) ease");
  assert.equal(rules.get(".stat__label")["letter-spacing"], "var(--shared-tracking-label)");
});

test("mobile layout tracks shrink below intrinsic content width", async () => {
  const css = postcss.parse(await defaultFile("base.css"));
  const selectors = [];
  css.walkAtRules("media", media => {
    if (media.params !== "(max-width: 48rem)") return;
    media.walkRules(rule => {
      rule.walkDecls("grid-template-columns", decl => {
        assert.equal(decl.value, "minmax(0, 1fr)");
        selectors.push(...rule.selectors);
      });
    });
  });
  assert.ok(selectors.includes(".shell"));
  assert.ok(selectors.includes(".grid--columns-2"));
});

test("open props subset defines every token default CSS references", async () => {
  const subset = await defaultFile("open-props.css");
  const own = (await Promise.all(["base.css", "default.css", "shared.css"].map(defaultFile))).join("\n");
  const all = subset + own;
  const defined = new Set([...all.matchAll(/(--[\w-]+)\s*:/g)].map(match => match[1]));
  const optional = new Set(["--gray-hue", "--gray-chroma"]);
  const missing = [...new Set([...all.matchAll(/var\(\s*(--[\w-]+)\s*\)/g)].map(match => match[1]))]
    .filter(name => !defined.has(name) && !optional.has(name));
  assert.deepEqual(missing, [], "undefined custom properties without fallback");
  assert.match(subset, /^\/\* Generated by npm run build:themes from open-props@/);
  assert.match(subset, /--indigo-7:/);
  assert.doesNotMatch(subset, /@import|@media/);
  const license = await defaultFile("LICENSE-open-props.txt");
  assert.match(license, /MIT License|Permission is hereby granted/);
});
for (const name of ["bootstrap", "bulma", "pico"]) {
  test(`${name} adapter maps semantic raised, subtle, and muted roles`, async () => {
    const source = await readFile(new URL(`theme/${name}/adapter.css`, root), "utf8");
    for (const role of ["--surface-raised", "--subtle", "--muted"]) {
      assert.match(source, new RegExp(`${role}:\\s*var\\(`), `missing ${role} mapping`);
    }
  });

  test(`${name} bundle has inline-safe assets, layout overrides, and upstream license`, async () => {
    const text = await readFile(new URL(`theme/${name}/framework.css`, root), "utf8");
    const css = postcss.parse(text);
    assert.doesNotMatch(text, /<\/style/i);
    assert.doesNotMatch(text, /sourceMappingURL=/);
    css.walkAtRules("import", rule => assert.fail(`external import: ${rule.params}`));
    css.walkDecls(decl => assert.doesNotMatch(decl.value, /url\(\s*["']?(?:https?:|\/\/)/i));
    const selectors = [];
    css.walkRules(rule => selectors.push(rule.selector));
    assert.ok(selectors.some(selector => selector.includes(".button--primary")));
    if (name === "bootstrap" || name === "pico") {
      const focus = css.nodes.findLast(node =>
        node.type === "rule" && node.selectors.includes(".button:focus-visible"));
      assert.ok(focus, "missing adapter keyboard focus fallback");
      assert.ok(focus.nodes.some(node =>
        node.prop === "outline" && node.value === "2px solid var(--primary)"));
    }
    if (name === "bootstrap") {
      assert.ok(selectors.some(selector => selector.includes("select.input")));
      css.walkRules(rule => {
        if (rule.selector.includes(".form-select") && rule.selector.includes("select.input")) {
          assert.ok(!rule.selector.split(",").some(selector => selector.trim() === ".input"),
            "Bootstrap select rules must not add dropdown arrows to text inputs");
        }
      });
    }
    assert.ok(selectors.includes(".grid--columns-12"));
    assert.ok(selectors.some(selector => selector.includes("data-theme")));
    let systemDark = false;
    css.walkAtRules("media", rule => {
      if (rule.params === "(prefers-color-scheme: dark)" && rule.toString().includes('[data-theme="system"]')) systemDark = true;
    });
    assert.ok(systemDark, "missing system-mode dark rules");
    const lastGrid = selectors.lastIndexOf(".grid");
    assert.ok(lastGrid > 0, "missing final framework-neutral layout");
    const license = await readFile(new URL(`theme/${name}/LICENSE.txt`, root), "utf8");
    assert.match(license, /MIT License|Permission is hereby granted/);
  });
}

test("shared widget CSS targets only component classes or html tokens", async () => {
  const css = postcss.parse(await defaultFile("shared.css"));
  const offenders = [];
  css.walkRules(rule => {
    if (rule.parent?.type === "atrule" && /keyframes$/.test(rule.parent.name)) return;
    for (const selector of rule.selectors) {
      if (selector.trim() === "html") continue;
      if (!/\.[a-z]/.test(selector)) offenders.push(selector);
    }
  });
  assert.deepEqual(offenders, [], "global element selectors leak into apps");
});

test("default CSS supports system mode, public tokens, and reduced motion", async () => {
  const text = (await Promise.all(["base.css", "default.css", "shared.css"].map(defaultFile))).join("\n");
  const css = postcss.parse(text);
  let systemMedia = 0;
  let reducedMotion = false;
  css.walkAtRules("media", rule => {
    if (rule.params === "(prefers-color-scheme: dark)" && rule.toString().includes('[data-theme="system"]')) systemMedia++;
    if (rule.params === "(prefers-reduced-motion: reduce)") reducedMotion = true;
  });
  assert.ok(systemMedia >= 2, "missing base and refinement system palettes");
  assert.ok(reducedMotion, "missing reduced-motion rule");
  const forcedColors = css.nodes.some(node => node.type === "atrule" && node.params === "(forced-colors: active)" &&
    node.toString().includes("Highlight") && node.toString().includes("!important"));
  assert.ok(forcedColors, "missing visible focus treatment in forced-colors mode");
  for (const token of ["--color-background", "--color-surface", "--color-surface-raised", "--color-surface-subtle", "--color-text", "--color-text-muted", "--color-border", "--color-primary", "--color-primary-text", "--color-info", "--color-success", "--color-warning", "--color-danger", "--color-danger-text", "--focus-ring", "--space-4", "--radius-medium"]) {
    assert.match(text, new RegExp(`${token}:`), `missing token ${token}`);
  }
});