import { spawn } from "node:child_process";
import { EventEmitter, once } from "node:events";
import path from "node:path";
import readline from "node:readline";

import { WorkerActionError } from "./errors.mjs";
import { generatePolicy } from "./policy.mjs";

function stoppedStatus(serverNumber) {
  return {
    serverNumber,
    profile: null,
    state: "stopped",
    pid: null,
    lastError: null
  };
}

function workerEnvironment(baseEnvironment, server, configPath) {
  const environment = { ...baseEnvironment };
  for (const key of Object.keys(environment)) {
    if (
      /^HLL_(?:HOST|PORT|PASSWORD|SERVER_LABEL)(?:_\d+)?$/.test(key)
      || key === "DISCORD_TOKEN"
      || key === "DISCORD_GUILD_ID"
      || /^GEOFENCE_(?:CHANNEL|LOG_CHANNEL|ERROR_LOG_CHANNEL)_/.test(key)
      || key === "GEOFENCE_LOG_CHANNEL_ID"
      || key === "GEOFENCE_ERROR_LOG_CHANNEL_ID"
      || key === "ALLOWED_USER_IDS"
      || key === "ALLOWED_ROLE_IDS"
    ) {
      delete environment[key];
    }
  }
  return {
    ...environment,
    HLL_HOST: server.host,
    HLL_PORT: `${server.port}`,
    HLL_PASSWORD: server.password,
    COMMUNITY_NAME: server.label,
    CONFIG_PATH: configPath
  };
}

export class WorkerSupervisor extends EventEmitter {
  constructor({
    config,
    stateStore,
    baseEnvironment = process.env,
    spawnImpl = spawn,
    generatePolicyImpl = generatePolicy
  }) {
    super();
    this.config = config;
    this.stateStore = stateStore;
    this.baseEnvironment = baseEnvironment;
    this.spawnImpl = spawnImpl;
    this.generatePolicyImpl = generatePolicyImpl;
    this.entries = new Map();
    this.statuses = new Map(config.servers.map((server) => [
      server.number,
      stoppedStatus(server.number)
    ]));
    this.queues = new Map();
    this.stateQueue = Promise.resolve();
  }

  server(serverNumber) {
    const server = this.config.servers.find((candidate) => candidate.number === serverNumber);
    if (!server) {
      throw new WorkerActionError("Invalid geofence server number.", "INVALID_SERVER");
    }
    return server;
  }

  status(serverNumber) {
    this.server(serverNumber);
    return { ...this.statuses.get(serverNumber) };
  }

  allStatuses() {
    return this.config.servers.map((server) => this.status(server.number));
  }

  exclusive(serverNumber, operation) {
    const previous = this.queues.get(serverNumber) ?? Promise.resolve();
    const current = previous.catch(() => {}).then(operation);
    this.queues.set(serverNumber, current);
    return current.finally(() => {
      if (this.queues.get(serverNumber) === current) {
        this.queues.delete(serverNumber);
      }
    });
  }

  async persist(serverNumber, profile) {
    const update = this.stateQueue.catch(() => {}).then(async () => {
      const existing = await this.stateStore.load() ?? { version: 1, servers: {} };
      existing.servers[`${serverNumber}`] = { profile };
      await this.stateStore.save(existing);
    });
    this.stateQueue = update;
    await update;
  }

  start(serverNumber, profile) {
    return this.exclusive(serverNumber, async () => {
      const server = this.server(serverNumber);
      const maxPlayers = this.config.profileLimits[profile];
      if (!Number.isInteger(maxPlayers)) {
        throw new WorkerActionError("Invalid geofence profile.", "INVALID_PROFILE");
      }

      await this.stopInternal(serverNumber, false);
      const outputPath = path.join(this.config.configDirectory, `server-${serverNumber}.yml`);
      await this.generatePolicyImpl({
        applyMapPath: this.config.applyMapBinary,
        outputPath,
        profile,
        maxPlayers
      });

      const child = this.spawnImpl(this.config.workerBinary, [], {
        env: workerEnvironment(this.baseEnvironment, server, outputPath),
        stdio: ["ignore", "pipe", "pipe"]
      });
      const entry = { child, profile, stopping: false, ready: false };
      this.entries.set(serverNumber, entry);
      this.attachLines(serverNumber, child.stdout, "stdout");
      this.attachLines(serverNumber, child.stderr, "stderr");
      child.once("close", (code, signal) => {
        if (this.entries.get(serverNumber) !== entry || entry.stopping) {
          return;
        }
        const message = `Worker exited unexpectedly (code=${code}, signal=${signal ?? "none"}).`;
        this.entries.delete(serverNumber);
        this.statuses.set(serverNumber, {
          serverNumber,
          profile,
          state: "failed",
          pid: null,
          lastError: message
        });
        this.emit("worker-error", { serverNumber, profile, message });
        this.emit("worker-status", this.status(serverNumber));
      });

      try {
        await once(child, "spawn");
      } catch (error) {
        this.entries.delete(serverNumber);
        throw new WorkerActionError(
          `Could not start server ${serverNumber} worker: ${error.message}`,
          "WORKER_START_FAILED",
          "The geofence worker could not be started."
        );
      }

      this.statuses.set(serverNumber, {
        serverNumber,
        profile,
        state: "running",
        pid: child.pid ?? null,
        lastError: null
      });
      await this.persist(serverNumber, profile);
      this.emit("worker-status", this.status(serverNumber));
      return this.status(serverNumber);
    });
  }

  attachLines(serverNumber, stream, source) {
    if (!stream) {
      return;
    }
    const lines = readline.createInterface({ input: stream, crlfDelay: Infinity });
    lines.on("line", (line) => {
      if (/(?:^| )msg=session-status(?: |$)/.test(line)) {
        const entry = this.entries.get(serverNumber);
        if (entry) entry.ready = true;
      }
      this.emit("worker-line", { serverNumber, source, line });
    });
  }

  stop(serverNumber) {
    return this.exclusive(serverNumber, () => this.stopInternal(serverNumber, true));
  }

  refresh(serverNumber) {
    return this.exclusive(serverNumber, async () => {
      this.server(serverNumber);
      const entry = this.entries.get(serverNumber);
      if (!entry || this.statuses.get(serverNumber)?.state !== "running") {
        throw new WorkerActionError(
          "The geofence worker is not running.",
          "WORKER_NOT_RUNNING"
        );
      }
      if (!entry.ready) {
        throw new WorkerActionError(
          "The geofence worker has not completed its first RCON status request.",
          "WORKER_NOT_READY",
          "Live RCON status is not available yet."
        );
      }
      const record = new Promise((resolve, reject) => {
        const handler = (candidate) => {
          if (
            candidate.serverNumber === serverNumber
            && /(?:^| )msg=session-status(?: |$)/.test(candidate.line)
          ) {
            clearTimeout(timer);
            this.off("worker-line", handler);
            resolve(candidate);
          }
        };
        const timer = setTimeout(() => {
          this.off("worker-line", handler);
          reject(new WorkerActionError(
            `Server ${serverNumber} status refresh timed out.`,
            "STATUS_REFRESH_TIMEOUT",
            "The live RCON status refresh timed out."
          ));
        }, this.config.statusRefreshTimeoutMs ?? 8_000);
        this.on("worker-line", handler);
      });
      if (!entry.child.kill("SIGUSR1")) {
        throw new WorkerActionError(
          `Could not signal server ${serverNumber} worker.`,
          "STATUS_REFRESH_SIGNAL_FAILED"
        );
      }
      return record;
    });
  }

  async stopInternal(serverNumber, persist) {
    this.server(serverNumber);
    const entry = this.entries.get(serverNumber);
    if (entry) {
      entry.stopping = true;
      const closed = once(entry.child, "close").catch(() => []);
      entry.child.kill("SIGTERM");
      const timedOut = await Promise.race([
        closed.then(() => false),
        new Promise((resolve) => {
          const timer = setTimeout(() => resolve(true), this.config.workerShutdownMs);
          timer.unref?.();
        })
      ]);
      if (timedOut) {
        entry.child.kill("SIGKILL");
        await closed;
      }
      this.entries.delete(serverNumber);
    }
    this.statuses.set(serverNumber, stoppedStatus(serverNumber));
    if (persist) {
      await this.persist(serverNumber, null);
    }
    this.emit("worker-status", this.status(serverNumber));
    return this.status(serverNumber);
  }

  async restore() {
    const state = await this.stateStore.load();
    for (const server of this.config.servers) {
      const profile = state
        ? (state.servers[`${server.number}`]?.profile ?? null)
        : this.config.defaultProfile;
      if (profile) {
        await this.start(server.number, profile);
      }
    }
  }

  async shutdown() {
    await Promise.all(this.config.servers.map((server) =>
      this.exclusive(server.number, () => this.stopInternal(server.number, false))
    ));
  }
}
