import { spawn, spawnSync } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { fileURLToPath } from "node:url";
import net from "node:net";

const root = resolve(fileURLToPath(new URL("../", import.meta.url)));
const temporaryDirectory = await mkdtemp(join(tmpdir(), "go-mvu-themes-"));
const extension = process.platform === "win32" ? ".exe" : "";
const servers = [];

function buildExample(name, packagePath) {
  const binary = join(temporaryDirectory, `${name}${extension}`);
  const result = spawnSync("go", ["build", "-o", binary, packagePath], {
    cwd: root,
    stdio: "inherit"
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`go build ${packagePath} failed: ${result.status}`);
  return binary;
}

async function assertPortAvailable(port) {
  await new Promise((resolvePromise, reject) => {
    const server = net.createServer();
    server.once("error", error => reject(new Error(`port ${port} unavailable: ${error.message}`)));
    server.listen(port, "127.0.0.1", () => server.close(resolvePromise));
  });
}

function startServer(binary, port) {
  const child = spawn(binary, [], { cwd: root, stdio: "inherit" });
  const state = { child, port, exit: null };
  child.once("exit", (code, signal) => {
    state.exit = { code, signal };
  });
  child.once("error", error => {
    state.exit = { error };
  });
  servers.push(state);
  return state;
}

async function waitForServer(server) {
  const url = `http://localhost:${server.port}/`;
  const deadline = Date.now() + 60_000;
  while (Date.now() < deadline) {
    if (server.exit) {
      throw new Error(`gallery on port ${server.port} exited before readiness: ${JSON.stringify(server.exit)}`);
    }
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(1000) });
      if (response.ok) return;
    } catch {
      await delay(200);
    }
  }
  throw new Error(`gallery on port ${server.port} did not become ready: ${url}`);
}

async function stopServers() {
  await Promise.all(servers.map(({ child, exit }) => {
    if (exit || child.exitCode !== null) return Promise.resolve();
    const exited = new Promise(resolvePromise => child.once("exit", resolvePromise));
    child.kill();
    return exited;
  }));
}

async function runBrowserTests() {
  const child = spawn(process.execPath, ["--test", "scripts/themes.browser.test.mjs"], {
    cwd: root,
    env: {
      ...process.env,
      THEME_GALLERY_URL: "http://localhost:8082/",
      COMPONENT_GALLERY_URL: "http://localhost:8083/"
    },
    stdio: "inherit"
  });
  await new Promise((resolvePromise, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) => {
      if (code === 0) resolvePromise();
      else reject(new Error(`browser tests failed: ${signal ?? code}`));
    });
  });
}

try {
  const themeBinary = buildExample("themes", "./examples/themes");
  const componentsBinary = buildExample("components", "./examples/components");
  await Promise.all([assertPortAvailable(8082), assertPortAvailable(8083)]);
  await Promise.all([
    waitForServer(startServer(themeBinary, 8082)),
    waitForServer(startServer(componentsBinary, 8083))
  ]);
  await runBrowserTests();
} finally {
  await stopServers();
  await rm(temporaryDirectory, { recursive: true, force: true });
}
