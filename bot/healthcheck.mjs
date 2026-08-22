import { readFile } from "node:fs/promises";

const filePath = process.env.BOT_HEALTH_FILE
  ?? `${process.env.BOT_DATA_DIR ?? "/app/data"}/health.json`;

try {
  const health = JSON.parse(await readFile(filePath, "utf8"));
  const updatedAt = Date.parse(`${health.updatedAt ?? ""}`);
  if (
    health.ready !== true
    || !Number.isFinite(updatedAt)
    || Date.now() - updatedAt > 10 * 60_000
  ) {
    throw new Error("bot health is not ready or is stale");
  }
} catch (error) {
  console.error(`Geofence bot healthcheck failed: ${error.message}`);
  process.exit(1);
}
