import { execFile } from "node:child_process";
import { chmod, mkdir, rename, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";

import { WorkerActionError } from "./errors.mjs";

const execFileAsync = promisify(execFile);
const PROFILES = new Set(["midcap", "midcap-advanced", "lastcap"]);

export async function generatePolicy({
  applyMapPath,
  outputPath,
  profile,
  maxPlayers,
  execFileImpl = execFileAsync
}) {
  if (!PROFILES.has(profile)) {
    throw new WorkerActionError("Invalid geofence profile.", "INVALID_PROFILE");
  }
  if (!Number.isInteger(maxPlayers) || maxPlayers < 0 || maxPlayers > 100) {
    throw new WorkerActionError(
      "Invalid inclusive player limit.",
      "INVALID_PLAYER_LIMIT"
    );
  }

  const directory = path.dirname(outputPath);
  await mkdir(directory, { recursive: true, mode: 0o700 });
  const temporaryPath = `${outputPath}.tmp-${process.pid}-${Date.now()}`;
  try {
    await writeFile(
      temporaryPath,
      "# HLL: Vietnam geofence policy. Runtime settings and credentials come from environment variables.\n"
        + "Servers:\n"
        + "  - AxisFence: []\n"
        + "    AlliesFence: []\n",
      { mode: 0o600, flag: "wx" }
    );
    await execFileImpl(
      applyMapPath,
      [
        "--map", "vietnam",
        "--variant", profile,
        "--max-players", `${maxPlayers}`,
        temporaryPath
      ],
      { timeout: 45_000, maxBuffer: 1024 * 1024 }
    );
    await chmod(temporaryPath, 0o600);
    await rename(temporaryPath, outputPath);
  } catch (error) {
    throw new WorkerActionError(
      `Could not generate ${profile} policy: ${error.message}`,
      "POLICY_GENERATION_FAILED",
      "The geofence policy could not be generated."
    );
  } finally {
    await rm(temporaryPath, { force: true }).catch(() => {});
  }
  return outputPath;
}
