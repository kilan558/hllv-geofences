import assert from "node:assert/strict";
import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

import { JsonStateStore } from "../bot/state-store.mjs";
import { WorkerSupervisor } from "../bot/worker-supervisor.mjs";

function waitForWorkerLine(supervisor, predicate) {
  return new Promise((resolve) => {
    const handler = (record) => {
      if (!predicate(record)) return;
      supervisor.off("worker-line", handler);
      resolve(record);
    };
    supervisor.on("worker-line", handler);
  });
}

async function fixture(t, { readyMode = "both" } = {}) {
  const directory = await mkdtemp(path.join(os.tmpdir(), "hllv-supervisor-test-"));
  const workerPath = path.join(directory, "fake-worker.mjs");
  const environmentPath = path.join(directory, "worker-environment.json");
  await writeFile(workerPath, `#!/usr/bin/env node
import { writeFileSync } from "node:fs";
writeFileSync(process.env.TEST_ENV_PATH, JSON.stringify({
  host: process.env.HLL_HOST,
  port: process.env.HLL_PORT,
  password: process.env.HLL_PASSWORD,
  host2: process.env.HLL_HOST_2 ?? null,
  password2: process.env.HLL_PASSWORD_2 ?? null,
  configPath: process.env.CONFIG_PATH
}));
const emitInitialStatus = () => console.log('level=INFO msg=session-status server_name="Test Server" map=wdeva_warfare_day mode=Warfare player_count=12 max_player_count=100 queue_count=1 max_queue_count=6');
let controlHandlerInstalled = false;
const installControlHandler = () => {
  if (controlHandlerInstalled) return;
  controlHandlerInstalled = true;
  process.on('SIGUSR1', () => {
  console.log('level=INFO msg=session-status server_name="Test Server" map=wdeva_warfare_day mode=Warfare player_count=13 max_player_count=100 queue_count=1 max_queue_count=6');
  });
  console.log('level=INFO msg=control-ready');
};
const readyMode = process.env.TEST_READY_MODE ?? 'both';
process.on('SIGUSR2', () => {
  if (readyMode === 'status-first') installControlHandler();
  if (readyMode === 'control-first') emitInitialStatus();
});
if (readyMode === 'status-first') {
  emitInitialStatus();
} else if (readyMode === 'control-first') {
  installControlHandler();
} else {
  installControlHandler();
  emitInitialStatus();
}
setInterval(() => {}, 1000);
`, { mode: 0o755 });
  await chmod(workerPath, 0o755);
  t.after(() => rm(directory, { recursive: true, force: true }));

  const generated = [];
  const config = {
    servers: [
      {
        number: 1,
        label: "One",
        host: "192.0.2.10",
        port: 7779,
        password: "first-secret",
        channelId: "123456789012345679"
      },
      {
        number: 2,
        label: "Two",
        host: "192.0.2.11",
        port: 7789,
        password: "second-secret",
        channelId: "123456789012345680"
      }
    ],
    defaultProfile: "midcap",
    profileLimits: { midcap: 42, "midcap-advanced": 88, lastcap: 77 },
    workerBinary: workerPath,
    applyMapBinary: "/unused/applymap",
    configDirectory: path.join(directory, "configs"),
    statusRefreshTimeoutMs: 200,
    workerShutdownMs: 2_000
  };
  const stateStore = new JsonStateStore(path.join(directory, "state.json"));
  const supervisor = new WorkerSupervisor({
    config,
    stateStore,
    baseEnvironment: {
      ...process.env,
      TEST_ENV_PATH: environmentPath,
      TEST_READY_MODE: readyMode,
      HLL_HOST: "should-be-replaced",
      HLL_PORT: "1",
      HLL_PASSWORD: "should-be-replaced",
      HLL_HOST_2: "must-not-leak",
      HLL_PORT_2: "2",
      HLL_PASSWORD_2: "must-not-leak"
    },
    async generatePolicyImpl(options) {
      generated.push(options);
      await import("node:fs/promises").then(async ({ mkdir, writeFile }) => {
        await mkdir(path.dirname(options.outputPath), { recursive: true });
        await writeFile(options.outputPath, "Servers:\n  - AxisFence: []\n    AlliesFence: []\n");
      });
    }
  });
  t.after(() => supervisor.shutdown());
  const releaseReady = (serverNumber = 1) => {
    const entry = supervisor.entries.get(serverNumber);
    assert.ok(entry, `worker ${serverNumber} must be running`);
    assert.equal(entry.child.kill("SIGUSR2"), true);
  };
  return {
    directory,
    environmentPath,
    generated,
    stateStore,
    supervisor,
    config,
    releaseReady
  };
}

test("WorkerSupervisor starts a worker with only its own server tuple", async (t) => {
  const { environmentPath, generated, supervisor } = await fixture(t);
  const linePromise = waitForWorkerLine(
    supervisor,
    (record) => /(?:^| )msg=session-status(?: |$)/.test(record.line)
  );

  await supervisor.start(2, "midcap-advanced");
  const line = await linePromise;

  assert.match(line.line, /msg=session-status/);
  assert.equal(line.serverNumber, 2);
  assert.equal(supervisor.status(2).profile, "midcap-advanced");
  assert.equal(supervisor.status(2).state, "running");
  assert.equal(generated[0].profile, "midcap-advanced");
  assert.equal(generated[0].maxPlayers, 88);
  assert.deepEqual(JSON.parse(await readFile(environmentPath, "utf8")), {
    host: "192.0.2.11",
    port: "7789",
    password: "second-secret",
    host2: null,
    password2: null,
    configPath: generated[0].outputPath
  });
});

test("WorkerSupervisor switches profiles and persists an explicit stop", async (t) => {
  const { stateStore, supervisor } = await fixture(t);

  await supervisor.start(1, "midcap");
  const firstPid = supervisor.status(1).pid;
  await supervisor.start(1, "lastcap");

  assert.equal(supervisor.status(1).profile, "lastcap");
  assert.notEqual(supervisor.status(1).pid, firstPid);
  assert.deepEqual(await stateStore.load(), {
    version: 1,
    servers: { "1": { profile: "lastcap" } }
  });

  await supervisor.stop(1);
  assert.deepEqual(supervisor.status(1), {
    serverNumber: 1,
    profile: null,
    state: "stopped",
    pid: null,
    lastError: null
  });
  assert.deepEqual(await stateStore.load(), {
    version: 1,
    servers: { "1": { profile: null } }
  });
});

test("WorkerSupervisor restores active and stopped server states", async (t) => {
  const { stateStore, supervisor } = await fixture(t);
  await stateStore.save({
    version: 1,
    servers: {
      "1": { profile: "lastcap" },
      "2": { profile: null }
    }
  });

  await supervisor.restore();

  assert.equal(supervisor.status(1).state, "running");
  assert.equal(supervisor.status(1).profile, "lastcap");
  assert.equal(supervisor.status(2).state, "stopped");
});

test("WorkerSupervisor preserves state updates from concurrent servers", async (t) => {
  const { stateStore, supervisor } = await fixture(t);

  await Promise.all([
    supervisor.start(1, "midcap"),
    supervisor.start(2, "lastcap")
  ]);

  assert.deepEqual(await stateStore.load(), {
    version: 1,
    servers: {
      "1": { profile: "midcap" },
      "2": { profile: "lastcap" }
    }
  });
});

test("WorkerSupervisor requests and waits for an immediate worker status refresh", async (t) => {
  const { supervisor } = await fixture(t);
  const initialStatus = waitForWorkerLine(
    supervisor,
    (record) => /(?:^| )msg=session-status(?: |$)/.test(record.line)
  );
  await supervisor.start(1, "midcap");
  await initialStatus;

  const record = await supervisor.refresh(1);

  assert.equal(record.serverNumber, 1);
  assert.match(record.line, /player_count=13/);
});

test("WorkerSupervisor does not signal a worker before its control handler is ready", async (t) => {
  const { releaseReady, supervisor } = await fixture(t, { readyMode: "status-first" });
  const initialStatus = waitForWorkerLine(
    supervisor,
    (record) => /(?:^| )msg=session-status(?: |$)/.test(record.line)
  );
  const controlReady = waitForWorkerLine(
    supervisor,
    (record) => /(?:^| )msg=control-ready(?: |$)/.test(record.line)
  );
  await supervisor.start(1, "midcap");
  await initialStatus;

  await assert.rejects(
    supervisor.refresh(1),
    (error) => error?.code === "WORKER_NOT_READY"
  );

  releaseReady();
  await controlReady;
  const record = await supervisor.refresh(1);
  assert.match(record.line, /player_count=13/);
});

test("WorkerSupervisor waits for an initial status after its control handler is ready", async (t) => {
  const { releaseReady, supervisor } = await fixture(t, { readyMode: "control-first" });
  const controlReady = waitForWorkerLine(
    supervisor,
    (record) => /(?:^| )msg=control-ready(?: |$)/.test(record.line)
  );
  const initialStatus = waitForWorkerLine(
    supervisor,
    (record) => /(?:^| )msg=session-status(?: |$)/.test(record.line)
  );
  await supervisor.start(1, "midcap");
  await controlReady;

  await assert.rejects(
    supervisor.refresh(1),
    (error) => error?.code === "WORKER_NOT_READY"
  );

  releaseReady();
  await initialStatus;
  const record = await supervisor.refresh(1);
  assert.match(record.line, /player_count=13/);
});

test("JsonStateStore persists positive server numbers above ten", async (t) => {
  const directory = await mkdtemp(path.join(os.tmpdir(), "hllv-state-eleven-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const store = new JsonStateStore(path.join(directory, "state.json"));
  const state = { version: 1, servers: { "11": { profile: "midcap" } } };

  await store.save(state);

  assert.deepEqual(await store.load(), state);
});
