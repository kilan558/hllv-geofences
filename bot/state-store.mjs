import { mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import path from "node:path";

import { WorkerActionError } from "./errors.mjs";

const PROFILE_IDS = new Set(["midcap", "midcap-advanced", "lastcap", null]);

function validateState(value) {
  if (!value || value.version !== 1 || !value.servers || typeof value.servers !== "object") {
    throw new WorkerActionError("Invalid persisted bot state.", "INVALID_STATE");
  }
  for (const [serverNumber, server] of Object.entries(value.servers)) {
    const parsedServerNumber = Number(serverNumber);
    if (
      !/^[1-9]\d*$/.test(serverNumber)
      || !Number.isSafeInteger(parsedServerNumber)
      || !server
      || !PROFILE_IDS.has(server.profile)
    ) {
      throw new WorkerActionError("Invalid persisted bot state.", "INVALID_STATE");
    }
  }
  return value;
}

export class JsonStateStore {
  constructor(filePath) {
    this.filePath = filePath;
  }

  async load() {
    let contents;
    try {
      contents = await readFile(this.filePath, "utf8");
    } catch (error) {
      if (error.code === "ENOENT") {
        return null;
      }
      throw new WorkerActionError(
        `Could not read bot state: ${error.message}`,
        "STATE_READ_FAILED"
      );
    }
    try {
      return validateState(JSON.parse(contents));
    } catch (error) {
      if (error instanceof WorkerActionError) {
        throw error;
      }
      throw new WorkerActionError("Invalid persisted bot state JSON.", "INVALID_STATE");
    }
  }

  async save(state) {
    validateState(state);
    const directory = path.dirname(this.filePath);
    await mkdir(directory, { recursive: true, mode: 0o700 });
    const temporaryPath = `${this.filePath}.tmp-${process.pid}-${Date.now()}`;
    try {
      await writeFile(temporaryPath, `${JSON.stringify(state, null, 2)}\n`, {
        mode: 0o600,
        flag: "wx"
      });
      await rename(temporaryPath, this.filePath);
    } catch (error) {
      throw new WorkerActionError(
        `Could not save bot state: ${error.message}`,
        "STATE_WRITE_FAILED"
      );
    } finally {
      await rm(temporaryPath, { force: true }).catch(() => {});
    }
  }
}

async function writeJsonAtomic(filePath, value) {
  const directory = path.dirname(filePath);
  await mkdir(directory, { recursive: true, mode: 0o700 });
  const temporaryPath = `${filePath}.tmp-${process.pid}-${Date.now()}`;
  try {
    await writeFile(temporaryPath, `${JSON.stringify(value, null, 2)}\n`, {
      mode: 0o600,
      flag: "wx"
    });
    await rename(temporaryPath, filePath);
  } finally {
    await rm(temporaryPath, { force: true }).catch(() => {});
  }
}

export class PanelStateStore {
  constructor(filePath) {
    this.filePath = filePath;
  }

  async load() {
    try {
      const value = JSON.parse(await readFile(this.filePath, "utf8"));
      if (!value || typeof value !== "object" || Array.isArray(value)) {
        throw new Error("panel state is not an object");
      }
      for (const [serverNumber, messageId] of Object.entries(value)) {
        const parsedServerNumber = Number(serverNumber);
        if (
          !/^[1-9]\d*$/.test(serverNumber)
          || !Number.isSafeInteger(parsedServerNumber)
          || !/^\d{15,22}$/.test(`${messageId}`)
        ) {
          throw new Error("panel state contains invalid IDs");
        }
      }
      return value;
    } catch (error) {
      if (error.code === "ENOENT") return {};
      throw new WorkerActionError(
        `Could not load Discord panel state: ${error.message}`,
        "PANEL_STATE_READ_FAILED"
      );
    }
  }

  async save(value) {
    await writeJsonAtomic(this.filePath, value);
  }
}

export class HealthStore {
  constructor(filePath) {
    this.filePath = filePath;
  }

  async ready(reason = "ready") {
    await writeJsonAtomic(this.filePath, {
      ready: true,
      reason,
      updatedAt: new Date().toISOString()
    });
  }

  async failed(reason) {
    await writeJsonAtomic(this.filePath, {
      ready: false,
      reason: `${reason}`.slice(0, 500),
      updatedAt: new Date().toISOString()
    });
  }
}
