import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, readFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import test from "node:test";

import { generatePolicy } from "../bot/policy.mjs";

const execFileAsync = promisify(execFile);

test("generatePolicy atomically creates a credential-free policy with an inclusive limit", async (t) => {
  const directory = await mkdtemp(path.join(os.tmpdir(), "hllv-policy-test-"));
  const applyMapPath = path.join(directory, "applymap");
  const outputPath = path.join(directory, "server-1.yml");
  await execFileAsync("go", ["build", "-o", applyMapPath, "./cmd/applymap"], {
    cwd: path.resolve(".")
  });
  t.after(async () => {
    await import("node:fs/promises").then(({ rm }) => rm(directory, { recursive: true, force: true }));
  });

  await generatePolicy({
    applyMapPath,
    outputPath,
    profile: "midcap",
    maxPlayers: 42
  });

  const yaml = await readFile(outputPath, "utf8");
  assert.match(yaml, /player_count: 43/);
  assert.doesNotMatch(yaml, /(?:Host|Port|Password|PunishAfterSeconds|Messages|Grid):/);
  for (const prefix of ["wdeva", "wdevb", "wdevc", "wdevd", "wdeve", "wdevf"]) {
    assert.match(yaml, new RegExp(`${prefix}_warfare_day`));
  }
});

test("generatePolicy rejects an unknown profile before replacing an existing policy", async (t) => {
  const { mkdtemp, writeFile, rm } = await import("node:fs/promises");
  const directory = await mkdtemp(path.join(os.tmpdir(), "hllv-policy-invalid-"));
  const outputPath = path.join(directory, "server-1.yml");
  await writeFile(outputPath, "preserve-me\n", { mode: 0o600 });
  t.after(() => rm(directory, { recursive: true, force: true }));

  await assert.rejects(
    generatePolicy({
      applyMapPath: "/missing/applymap",
      outputPath,
      profile: "unknown",
      maxPlayers: 42
    }),
    /Invalid geofence profile/
  );
  assert.equal(await readFile(outputPath, "utf8"), "preserve-me\n");
});
