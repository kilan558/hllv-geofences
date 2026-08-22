import path from "node:path";

import { ConfigurationError } from "./errors.mjs";

const SNOWFLAKE = /^\d{15,22}$/;
const PROFILE_IDS = new Set(["midcap", "midcap-advanced", "lastcap"]);

function singleLine(env, key, { required = true, fallback = "", trim = true } = {}) {
  let value = env?.[key] ?? fallback;
  value = `${value}`;
  if (value.includes("\n") || value.includes("\r")) {
    throw new ConfigurationError(`${key} must not contain line breaks.`);
  }
  if (trim) {
    value = value.trim();
  }
  if (required && !value.trim()) {
    throw new ConfigurationError(`Missing required environment variable: ${key}.`);
  }
  return value;
}

function integer(env, key, fallback, minimum, maximum) {
  const raw = singleLine(env, key, { fallback: `${fallback}` });
  if (!/^\d+$/.test(raw)) {
    throw new ConfigurationError(`${key} must be an integer between ${minimum} and ${maximum}.`);
  }
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value < minimum || value > maximum) {
    throw new ConfigurationError(`${key} must be an integer between ${minimum} and ${maximum}.`);
  }
  return value;
}

function snowflake(env, key) {
  const value = singleLine(env, key);
  if (!SNOWFLAKE.test(value)) {
    throw new ConfigurationError(`${key} must be a valid Discord ID.`);
  }
  return value;
}

function optionalSnowflake(env, key) {
  const value = singleLine(env, key, { required: false });
  if (!value) return null;
  if (!SNOWFLAKE.test(value)) {
    throw new ConfigurationError(`${key} must be a valid Discord ID when configured.`);
  }
  return value;
}

function snowflakeList(env, key) {
  const raw = singleLine(env, key, { required: false });
  const values = raw.split(",").map((value) => value.trim()).filter(Boolean);
  if (values.some((value) => !SNOWFLAKE.test(value))) {
    throw new ConfigurationError(`${key} must contain only comma-separated Discord IDs.`);
  }
  return Object.freeze([...new Set(values)]);
}

function discoverServerNumbers(env) {
  const numbers = new Set([1]);
  for (const key of Object.keys(env ?? {})) {
    const match = key.match(/^HLL_(?:HOST|PORT|PASSWORD)_(\d+)$/);
    if (!match) {
      continue;
    }
    const number = Number(match[1]);
    if (!Number.isSafeInteger(number) || number < 2) {
      throw new ConfigurationError(`${key} has an invalid server suffix; use _2, _3, and so on.`);
    }
    numbers.add(number);
  }
  const sorted = [...numbers].sort((left, right) => left - right);
  for (let index = 0; index < sorted.length; index += 1) {
    const expected = index + 1;
    if (sorted[index] !== expected) {
      throw new ConfigurationError(
        `server numbering must be contiguous; expected server ${expected}, found server ${sorted[index]}.`
      );
    }
  }
  return sorted;
}

function loadServer(env, number) {
  const suffix = number === 1 ? "" : `_${number}`;
  return Object.freeze({
    number,
    label: singleLine(env, `HLL_SERVER_LABEL${suffix}`, {
      required: false,
      fallback: `Server ${number}`
    }) || `Server ${number}`,
    host: singleLine(env, `HLL_HOST${suffix}`),
    port: integer(env, `HLL_PORT${suffix}`, "", 1, 65_535),
    password: singleLine(env, `HLL_PASSWORD${suffix}`, { trim: false }),
    channelId: snowflake(env, `GEOFENCE_CHANNEL_S${number}_ID`)
  });
}

export function loadBotConfig(env = process.env) {
  const language = singleLine(env, "GEOFENCE_LANGUAGE", { fallback: "en" }).toLowerCase();
  if (!new Set(["en", "de"]).has(language)) {
    throw new ConfigurationError("GEOFENCE_LANGUAGE must be en or de.");
  }

  const defaultProfile = singleLine(env, "GEOFENCE_PROFILE", { fallback: "midcap" }).toLowerCase();
  if (!PROFILE_IDS.has(defaultProfile)) {
    throw new ConfigurationError(
      "GEOFENCE_PROFILE must be midcap, midcap-advanced, or lastcap."
    );
  }

  const profileLimits = Object.freeze({
    midcap: integer(env, "GEOFENCE_MIDCAP_MAX_PLAYERS", 60, 0, 100),
    "midcap-advanced": integer(
      env,
      "GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS",
      99,
      0,
      100
    ),
    lastcap: integer(env, "GEOFENCE_LASTCAP_MAX_PLAYERS", 90, 0, 100)
  });

  const servers = Object.freeze(discoverServerNumbers(env).map((number) => loadServer(env, number)));
  const guildId = snowflake(env, "DISCORD_GUILD_ID");
  const logChannelId = optionalSnowflake(env, "GEOFENCE_LOG_CHANNEL_ID");
  const errorLogChannelId = optionalSnowflake(env, "GEOFENCE_ERROR_LOG_CHANNEL_ID");
  const channelIds = [
    ...servers.map((server) => server.channelId),
    logChannelId,
    errorLogChannelId
  ].filter(Boolean);
  if (new Set(channelIds).size !== channelIds.length) {
    throw new ConfigurationError("Discord channel IDs must be distinct.");
  }

  const dataDirectory = path.resolve(singleLine(env, "BOT_DATA_DIR", {
    fallback: "/app/data"
  }));
  const configDirectory = path.join(dataDirectory, "configs");

  return Object.freeze({
    token: singleLine(env, "DISCORD_TOKEN"),
    guildId,
    language,
    defaultProfile,
    profileLimits,
    servers,
    logChannelId,
    errorLogChannelId,
    allowedUserIds: snowflakeList(env, "ALLOWED_USER_IDS"),
    allowedRoleIds: snowflakeList(env, "ALLOWED_ROLE_IDS"),
    refreshIntervalMs: integer(env, "STATUS_REFRESH_SECONDS", 20, 10, 300) * 1_000,
    statusRefreshTimeoutMs: integer(
      env,
      "STATUS_REFRESH_TIMEOUT_SECONDS",
      8,
      2,
      30
    ) * 1_000,
    workerShutdownMs: integer(env, "WORKER_SHUTDOWN_SECONDS", 15, 1, 30) * 1_000,
    punishAfterSeconds: integer(env, "GEOFENCE_PUNISH_AFTER_SECONDS", 10, 1, 3_600),
    dataDirectory,
    configDirectory,
    stateFile: path.join(dataDirectory, "state.json"),
    panelStateFile: path.join(dataDirectory, "panels.json"),
    healthFile: path.join(dataDirectory, "health.json"),
    workerBinary: path.resolve(singleLine(env, "GEOFENCE_WORKER_BINARY", {
      fallback: "/usr/local/bin/hllv-geofences"
    })),
    applyMapBinary: path.resolve(singleLine(env, "GEOFENCE_APPLYMAP_BINARY", {
      fallback: "/usr/local/bin/applymap"
    }))
  });
}

export { ConfigurationError } from "./errors.mjs";
