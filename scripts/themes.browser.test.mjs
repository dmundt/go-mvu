import assert from "node:assert/strict";
import test, { after, before } from "node:test";
import { chromium } from "playwright";
import contract from "./theme-contract.json" with { type: "json" };

const themes = ["default", "bootstrap", "bulma", "pico"];
const modes = [
  { name: "light", colorScheme: "light" },
  { name: "dark", colorScheme: "dark" },
  { name: "system-light", mode: "system", colorScheme: "light" },
  { name: "system-dark", mode: "system", colorScheme: "dark" }
];
const requiredTokens = contract.requiredTokens;
const themeGallery = process.env.THEME_GALLERY_URL ?? "http://localhost:8082/";
const componentGallery = process.env.COMPONENT_GALLERY_URL ?? "http://localhost:8083/";

let browser;
before(async () => {
  const executablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
  browser = await chromium.launch({
    headless: true,
    ...(executablePath ? { executablePath } : {})
  });
});
after(async () => {
  await browser?.close();
});

async function readStyles(page, baseURL, theme, mode, viewport = { width: 1280, height: 900 }) {
  const url = new URL(baseURL);
  url.searchParams.set("theme", theme);
  url.searchParams.set("mode", mode.mode ?? mode.name);
  await page.setViewportSize(viewport);
  await page.emulateMedia({ colorScheme: mode.colorScheme });
  const response = await page.goto(url.href, { waitUntil: "domcontentloaded" });
  assert.equal(response.status(), 200, `${url.href} response status`);
  if (baseURL === themeGallery) {
    const primaryButton = page.locator(".button--primary");
    for (let index = 0; index < 60; index++) {
      if (await primaryButton.evaluate(element => element === document.activeElement)) break;
      await page.keyboard.press("Tab");
    }
    assert.equal(
      await primaryButton.evaluate(element => element === document.activeElement),
      true,
      `${url.href}: primary button reachable by keyboard`
    );
  }
  return await page.evaluate(tokensToCheck => {
      const style = element => getComputedStyle(element);
      const canvas = document.createElement("canvas");
      const context = canvas.getContext("2d", { willReadFrequently: true });
      const colorChannels = color => {
        context.clearRect(0, 0, 1, 1);
        context.fillStyle = color;
        context.fillRect(0, 0, 1, 1);
        return [...context.getImageData(0, 0, 1, 1).data].map(value => value / 255);
      };
      const luminance = color => {
        const channels = (Array.isArray(color) ? color : colorChannels(color).slice(0, 3)).map(value =>
          value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
        );
        return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
      };
      const contrast = (foreground, background) => {
        const [first, second] = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
        return (first + 0.05) / (second + 0.05);
      };
      const composite = (foreground, background) => {
        const channels = colorChannels(foreground);
        return channels.slice(0, 3).map((channel, index) =>
          channel * channels[3] + background[index] * (1 - channels[3])
        );
      };
      const effectiveBackground = element => {
        const chain = [];
        for (let current = element; current && current !== document.body; current = current.parentElement) {
          chain.push(style(current).backgroundColor);
        }
        return chain.reverse().reduce(
          (background, color) => composite(color, background),
          colorChannels(body.backgroundColor).slice(0, 3)
        );
      };
      const html = document.documentElement;
      const rootStyle = style(html);
      const buttonElement = document.querySelector(".button--primary");
      const inputElement = document.querySelector(".input");
      const badgeElement = document.querySelector(".badge");
      const alertElement = document.querySelector(".alert--success");
      const button = buttonElement && style(buttonElement);
      const input = inputElement && style(inputElement);
      const body = style(document.body);
      const badge = badgeElement && style(badgeElement);
      const alert = alertElement && style(alertElement);
      const inputBackground = inputElement && effectiveBackground(inputElement);
      const buttonFocus = buttonElement && style(buttonElement);
      const tableElement = document.querySelector("table");
      const headerElement = document.querySelector("thead th");
      const cellElement = document.querySelector("tbody td");
      const table = tableElement && style(tableElement);
      const header = headerElement && style(headerElement);
      const cell = cellElement && style(cellElement);
      const mutedProbe = document.createElement("span");
      mutedProbe.style.color = "rgb(1, 2, 3)";
      document.body.append(mutedProbe);
      const mutedBaseline = style(mutedProbe).color;
      mutedProbe.style.color = "var(--color-text-muted)";
      const mutedColor = style(mutedProbe).color;
      const tokenProbe = document.createElement("span");
      tokenProbe.style.cssText = "position:absolute;visibility:hidden";
      document.body.append(tokenProbe);
      const tokenProperties = Object.fromEntries(tokensToCheck.map(name => {
        if (name.startsWith("--color-")) return [name, "color"];
        if (name === "--focus-ring" || name.startsWith("--shadow-")) return [name, "box-shadow"];
        if (name.startsWith("--radius-")) return [name, "border-radius"];
        if (name.startsWith("--space-")) return [name, "padding"];
        return [name, null];
      }));
      const invalidTokens = [];
      for (const [name, property] of Object.entries(tokenProperties)) {
        tokenProbe.style.setProperty(property, "rgb(1, 2, 3)");
        const baseline = style(tokenProbe).getPropertyValue(property);
        tokenProbe.style.setProperty(property, `var(${name})`);
        if (!style(tokenProbe).getPropertyValue(property) ||
            style(tokenProbe).getPropertyValue(property) === baseline) {
          invalidTokens.push(name);
        }
      }
      mutedProbe.remove();
      tokenProbe.remove();
      const stackElement = document.querySelector(".stack");
      const grids = [...document.querySelectorAll(".grid")];
      const shell = document.createElement("div");
      shell.className = "shell";
      shell.innerHTML = "<aside></aside><main></main>";
      shell.style.cssText = "position:absolute;visibility:hidden";
      document.body.append(shell);
      const shellStyle = style(shell);
      const result = {
        themeName: html.dataset.themeName,
        mode: html.dataset.theme,
        colorScheme: rootStyle.colorScheme,
        background: body.backgroundColor,
        text: body.color,
        contrast: {
          body: contrast(body.color, body.backgroundColor),
          button: contrast(button.color, button.backgroundColor),
          input: input && contrast(input.color, inputBackground),
          muted: contrast(mutedColor, body.backgroundColor),
          badge: badge && contrast(badge.color, effectiveBackground(badgeElement)),
          alert: alert && contrast(alert.color, effectiveBackground(alertElement)),
          tableHeader: header && contrast(header.color, effectiveBackground(headerElement))
        },
        mutedResolved: mutedColor !== mutedBaseline,
        invalidTokens,
        focus: {
          visible: buttonElement?.matches(":focus-visible") ?? false,
          outlineStyle: buttonFocus?.outlineStyle,
          outlineWidth: buttonFocus ? parseFloat(buttonFocus.outlineWidth) : 0,
          boxShadow: buttonFocus?.boxShadow
        },
        documentWidth: document.documentElement.scrollWidth,
        viewportWidth: window.innerWidth,
        tokens: Object.fromEntries(
          tokensToCheck.map(name => [name, rootStyle.getPropertyValue(name).trim()])
        ),
        button: {
          background: button?.backgroundColor,
          color: button?.color,
          minHeight: button ? parseFloat(button.minHeight) : 0,
          height: buttonElement?.getBoundingClientRect().height ?? 0,
          width: buttonElement?.getBoundingClientRect().width ?? 0
        },
        input: {
          borderWidth: input ? parseFloat(input.borderTopWidth) : 0,
          borderStyle: input?.borderTopStyle,
          borderRadius: input ? parseFloat(input.borderTopLeftRadius) : 0,
          color: input?.color,
          height: inputElement?.getBoundingClientRect().height ?? 0,
          width: inputElement?.getBoundingClientRect().width ?? 0
        },
        table: table && {
          width: parseFloat(table.width),
          borderCollapse: table.borderCollapse,
          headerBackground: header.backgroundColor,
          cellBorderWidth: parseFloat(cell.borderBottomWidth),
          cellBorderStyle: cell.borderBottomStyle
        },
        stack: stackElement && {
          display: style(stackElement).display,
          gap: parseFloat(style(stackElement).rowGap)
        },
        grids: grids.map(element => {
          const grid = style(element);
          return {
            display: grid.display,
            columns: grid.gridTemplateColumns.trim().split(/\s+/).length,
            gap: parseFloat(grid.columnGap)
          };
        }),
        shell: {
          display: shellStyle.display,
          columns: shellStyle.gridTemplateColumns
        }
      };
      shell.remove();
      return result;
  }, requiredTokens);
}

for (const theme of themes) {
  test(`${theme} adapter computed styles satisfy shared contract`, async () => {
    const themePage = await browser.newPage();
    const componentPage = await browser.newPage();
    const pageStyles = [];
    try {
      for (const mode of modes) {
        pageStyles.push([
          await readStyles(themePage, themeGallery, theme, mode),
          await readStyles(componentPage, componentGallery, theme, mode)
        ]);
      }
    } finally {
      await Promise.all([themePage.close(), componentPage.close()]);
    }
    const [light, dark, systemLight, systemDark] = pageStyles.map(([gallery]) => gallery);

    for (const [index, mode] of modes.entries()) {
      const [styles, components] = pageStyles[index];
      const modeName = mode.name;
      assert.equal(styles.themeName, theme, `${modeName}: selected theme`);
      assert.ok(styles.background && styles.text, `${modeName}: body colors`);
      assert.equal(styles.mutedResolved, true, `${modeName}: muted text token resolves`);
      assert.deepEqual(styles.invalidTokens, [], `${modeName}: tokens are valid in consumer properties`);
      for (const token of requiredTokens) {
        assert.ok(styles.tokens[token], `${modeName}: missing computed token ${token}`);
      }
      assert.notEqual(styles.button.background, "rgba(0, 0, 0, 0)", `${modeName}: button background`);
      assert.ok(styles.button.color, `${modeName}: button text`);
      assert.ok(styles.button.minHeight >= 40, `${modeName}: button min-height`);
      assert.ok(styles.contrast.body >= 4.5, `${modeName}: body text contrast ${styles.contrast.body}`);
      assert.ok(styles.contrast.button >= 4.5, `${modeName}: button text contrast ${styles.contrast.button}`);
      assert.ok(styles.contrast.input >= 4.5, `${modeName}: input text contrast ${styles.contrast.input}`);
      assert.ok(styles.contrast.muted >= 4.5, `${modeName}: muted text contrast ${styles.contrast.muted}`);
      assert.ok(styles.contrast.badge >= 4.5, `${modeName}: badge contrast ${styles.contrast.badge}`);
      assert.ok(styles.contrast.alert >= 4.5, `${modeName}: alert contrast ${styles.contrast.alert}`);
      assert.ok(
        styles.contrast.tableHeader >= 4.5,
        `${modeName}: table header contrast ${styles.contrast.tableHeader}`
      );
      assert.equal(styles.focus.visible, true, `${modeName}: keyboard focus-visible`);
      assert.ok(
        (styles.focus.outlineStyle !== "none" && styles.focus.outlineWidth > 0) ||
        styles.focus.boxShadow !== "none",
        `${modeName}: visible focus indicator`
      );
      assert.ok(styles.input.borderWidth > 0, `${modeName}: input border width`);
      assert.notEqual(styles.input.borderStyle, "none", `${modeName}: input border style`);
      assert.ok(styles.input.borderRadius > 0, `${modeName}: input border radius`);
      assert.ok(styles.table, `${modeName}: semantic table rendered`);
      assert.ok(styles.table.width > 0, `${modeName}: table width`);
      assert.equal(styles.table.borderCollapse, "collapse", `${modeName}: table borders`);
      assert.ok(styles.table.headerBackground !== "rgba(0, 0, 0, 0)", `${modeName}: table header surface`);
      assert.ok(styles.table.cellBorderWidth > 0, `${modeName}: table cell border`);
      assert.notEqual(styles.table.cellBorderStyle, "none", `${modeName}: table cell border style`);
      assert.equal(styles.shell.display, "grid", `${modeName}: shell layout`);
      assert.notEqual(styles.shell.columns, "none", `${modeName}: shell columns`);
      assert.equal(components.stack?.display, "flex", `${modeName}: stack display`);
      assert.ok(components.stack?.gap > 0, `${modeName}: stack gap`);
      assert.ok(components.grids.length > 0, `${modeName}: gallery grids rendered`);
      for (const grid of components.grids) {
        assert.equal(grid.display, "grid", `${modeName}: grid display`);
        assert.ok(grid.columns > 0, `${modeName}: grid columns`);
        assert.ok(grid.gap > 0, `${modeName}: grid gap`);
      }
    }
    assert.notEqual(light.background, dark.background, "explicit light/dark palettes differ");
    assert.notEqual(systemLight.background, systemDark.background, "system mode follows OS scheme");

    const mobileThemePage = await browser.newPage();
    const mobileComponentPage = await browser.newPage();
    try {
      for (const viewportWidth of [390, 320]) {
        const viewport = { width: viewportWidth, height: 844 };
        const systemLight = { name: "system", mode: "system", colorScheme: "light" };
        const mobileTheme = await readStyles(mobileThemePage, themeGallery, theme, systemLight, viewport);
        const mobileComponents = await readStyles(mobileComponentPage, componentGallery, theme, systemLight, viewport);
        for (const [baseURL, mobile] of [
          [themeGallery, mobileTheme],
          [componentGallery, mobileComponents]
        ]) {
          assert.ok(
            mobile.documentWidth <= mobile.viewportWidth,
            `${viewportWidth}px ${baseURL}: horizontal page overflow (${mobile.documentWidth}px)`
          );
          assert.ok(mobile.button.height >= 40, `${viewportWidth}px ${baseURL}: usable button height`);
          assert.ok(mobile.button.width >= 40, `${viewportWidth}px ${baseURL}: usable button width`);
          assert.ok(mobile.input.height >= 40, `${viewportWidth}px ${baseURL}: usable input height`);
          assert.ok(mobile.input.width > 0, `${viewportWidth}px ${baseURL}: input rendered`);
        }
        assert.ok(mobileComponents.grids.length > 0, `${viewportWidth}px: gallery grids rendered`);
        assert.ok(
          mobileComponents.grids.every(grid => grid.columns === 1),
          `${viewportWidth}px: all grids collapse to one column`
        );
      }
    } finally {
      await Promise.all([mobileThemePage.close(), mobileComponentPage.close()]);
    }
  });

  for (const colorScheme of ["light", "dark"]) {
    test(`${theme} ${colorScheme} shared widgets preserve accessible keyboard behavior`, async () => {
      for (const width of [390, 1280]) {
        const url = new URL(componentGallery);
        url.searchParams.set("theme", theme);
        url.searchParams.set("mode", colorScheme);
        const page = await browser.newPage({
          colorScheme,
          viewport: { width, height: 900 }
        });
        try {
          const response = await page.goto(url.href, { waitUntil: "networkidle" });
          assert.equal(response.status(), 200, `${theme}/${colorScheme}/${width}`);

          const navigation = page.getByRole("navigation", { name: "Gallery sections" });
          const overview = navigation.getByRole("link", { name: "Overview" });
          assert.equal(await overview.getAttribute("aria-current"), "page");
          assert.equal(await page.getByRole("region", { name: "Overview" }).count(), 1);
          await navigation.getByRole("link", { name: "Activity" }).focus();
          await page.keyboard.press("Enter");
          await page.waitForURL(/tab=activity/);
          assert.equal(
            await page.getByRole("navigation", { name: "Gallery sections" })
              .getByRole("link", { name: "Activity" }).getAttribute("aria-current"),
            "page"
          );
          assert.equal(await page.getByRole("region", { name: "Activity" }).count(), 1);

          const secondAccordionSummary = page.locator(".accordion__trigger").nth(1);
          await secondAccordionSummary.focus();
          await page.keyboard.press("Space");
          assert.equal(await page.locator(".accordion__item").nth(1).getAttribute("open") !== null, true);
          assert.equal(await page.locator(".accordion__item").nth(0).getAttribute("open"), null);

          const tooltipButton = page.getByRole("button", { name: "Show help" });
          const tooltip = page.locator("#save-tip");
          assert.equal(await tooltipButton.getAttribute("aria-describedby"), "save-tip");
          assert.equal(await tooltip.getAttribute("role"), "tooltip");
          await tooltipButton.focus();
          await page.keyboard.press("Space");
          await page.waitForFunction(() =>
            document.querySelector("#save-tip")?.matches(":popover-open")
          );
          const position = await tooltip.evaluate(element => {
            const rect = element.getBoundingClientRect();
            const trigger = document.querySelector('[popovertarget="save-tip"]')
              .getBoundingClientRect();
            const style = getComputedStyle(element);
            return {
              open: element.matches(":popover-open"),
              positionAnchor: style.positionAnchor,
              positionArea: style.positionArea,
              below: rect.top >= trigger.bottom,
              above: rect.bottom <= trigger.top,
              inViewport: rect.left >= 0 && rect.right <= innerWidth
            };
          });
          assert.equal(position.open, true);
          assert.equal(position.positionAnchor, "--tip");
          assert.ok(position.inViewport, "tooltip stays inside viewport");
          assert.ok(
            (position.positionArea === "block-end" && position.below) ||
            (position.positionArea === "block-start" && position.above),
            "tooltip uses valid below/above anchor placement"
          );
          await page.keyboard.press("Escape");
          assert.equal(await tooltip.evaluate(element => element.matches(":popover-open")), false);
          assert.equal(await tooltipButton.evaluate(element => document.activeElement === element), true);

          const dialogTrigger = page.getByRole("button", { name: "Open dialog" });
          await dialogTrigger.focus();
          await page.keyboard.press("Enter");
          const dialog = page.getByRole("dialog", { name: "Invite teammate" });
          await dialog.waitFor({ state: "visible" });
          await page.keyboard.press("Escape");
          await dialog.waitFor({ state: "hidden" });
          assert.equal(await dialogTrigger.evaluate(element => document.activeElement === element), true);
        } finally {
          await page.close();
        }
      }
    });
  }
}
