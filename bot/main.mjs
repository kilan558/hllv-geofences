import {
  Client,
  GatewayIntentBits
} from "discord.js";

import { loadBotConfig } from "./config.mjs";
import { DiscordGeofenceBot } from "./discord-bot.mjs";
import { HealthStore, JsonStateStore, PanelStateStore } from "./state-store.mjs";
import { WorkerSupervisor } from "./worker-supervisor.mjs";

function log(level, event, fields = {}) {
  const output = JSON.stringify({
    timestamp: new Date().toISOString(),
    level,
    application: "hllv-geofence-discord",
    event,
    ...fields
  });
  if (level === "error") console.error(output);
  else if (level === "warn") console.warn(output);
  else console.log(output);
}

const logger = {
  info(event, fields) { log("info", event, fields); },
  warn(event, fields) { log("warn", event, fields); },
  error(event, fields) { log("error", event, fields); }
};

let bot;
try {
  const config = loadBotConfig();
  const supervisor = new WorkerSupervisor({
    config,
    stateStore: new JsonStateStore(config.stateFile)
  });
  bot = new DiscordGeofenceBot({
    config,
    supervisor,
    client: new Client({ intents: [GatewayIntentBits.Guilds] }),
    panelStore: new PanelStateStore(config.panelStateFile),
    health: new HealthStore(config.healthFile),
    logger
  });
  await bot.run();
} catch (error) {
  logger.error("startup-failed", { error: error.message });
  process.exitCode = 1;
}

let shuttingDown = false;
for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, async () => {
    if (shuttingDown) return;
    shuttingDown = true;
    logger.info("shutdown-requested", { signal });
    await bot?.shutdown().catch((error) => {
      logger.error("shutdown-failed", { error: error.message });
      process.exitCode = 1;
    });
  });
}
