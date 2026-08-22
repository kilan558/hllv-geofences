import assert from "node:assert/strict";
import test from "node:test";

import { ConfigurationError, loadBotConfig } from "../bot/config.mjs";

const baseEnv = Object.freeze({
  DISCORD_TOKEN: "discord-token",
  DISCORD_GUILD_ID: "123456789012345678",
  GEOFENCE_CHANNEL_S1_ID: "123456789012345679",
  GEOFENCE_CHANNEL_S2_ID: "123456789012345680",
  GEOFENCE_LOG_CHANNEL_ID: "123456789012345681",
  GEOFENCE_ERROR_LOG_CHANNEL_ID: "123456789012345682",
  HLL_HOST: "192.0.2.10",
  HLL_PORT: "7779",
  HLL_PASSWORD: "first-secret",
  HLL_SERVER_LABEL: "Vietnam One",
  HLL_HOST_2: "192.0.2.11",
  HLL_PORT_2: "7789",
  HLL_PASSWORD_2: "second-secret",
  HLL_SERVER_LABEL_2: "Vietnam Two",
  GEOFENCE_LANGUAGE: "de",
  GEOFENCE_PROFILE: "midcap",
  GEOFENCE_MIDCAP_MAX_PLAYERS: "42",
  GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS: "88",
  GEOFENCE_LASTCAP_MAX_PLAYERS: "77",
  GEOFENCE_PUNISH_AFTER_SECONDS: "17",
  STATUS_REFRESH_SECONDS: "25",
  WORKER_SHUTDOWN_SECONDS: "12",
  ALLOWED_USER_IDS: "123456789012345690,123456789012345690",
  ALLOWED_ROLE_IDS: "123456789012345691"
});

test("loadBotConfig maps each server tuple to its Discord channel once", () => {
  const config = loadBotConfig(baseEnv);

  assert.equal(config.language, "de");
  assert.equal(config.defaultProfile, "midcap");
  assert.deepEqual(config.profileLimits, {
    midcap: 42,
    "midcap-advanced": 88,
    lastcap: 77
  });
  assert.deepEqual(config.servers, [
    {
      number: 1,
      label: "Vietnam One",
      host: "192.0.2.10",
      port: 7779,
      password: "first-secret",
      channelId: "123456789012345679"
    },
    {
      number: 2,
      label: "Vietnam Two",
      host: "192.0.2.11",
      port: 7789,
      password: "second-secret",
      channelId: "123456789012345680"
    }
  ]);
  assert.deepEqual(config.allowedUserIds, ["123456789012345690"]);
  assert.deepEqual(config.allowedRoleIds, ["123456789012345691"]);
  assert.equal(config.refreshIntervalMs, 25_000);
  assert.equal(config.workerShutdownMs, 12_000);
  assert.equal(config.punishAfterSeconds, 17);
});

test("loadBotConfig uses English and documented profile defaults", () => {
  const env = { ...baseEnv };
  delete env.GEOFENCE_LANGUAGE;
  delete env.GEOFENCE_PROFILE;
  delete env.GEOFENCE_MIDCAP_MAX_PLAYERS;
  delete env.GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS;
  delete env.GEOFENCE_LASTCAP_MAX_PLAYERS;

  const config = loadBotConfig(env);

  assert.equal(config.language, "en");
  assert.equal(config.defaultProfile, "midcap");
  assert.deepEqual(config.profileLimits, {
    midcap: 60,
    "midcap-advanced": 99,
    lastcap: 90
  });
});

test("loadBotConfig allows both Discord log channels to be omitted independently", () => {
  const eventLogDisabled = { ...baseEnv };
  delete eventLogDisabled.GEOFENCE_LOG_CHANNEL_ID;
  const eventConfig = loadBotConfig(eventLogDisabled);

  assert.equal(eventConfig.logChannelId, null);
  assert.equal(eventConfig.errorLogChannelId, baseEnv.GEOFENCE_ERROR_LOG_CHANNEL_ID);

  const errorLogDisabled = { ...baseEnv, GEOFENCE_ERROR_LOG_CHANNEL_ID: "" };
  const errorConfig = loadBotConfig(errorLogDisabled);

  assert.equal(errorConfig.logChannelId, baseEnv.GEOFENCE_LOG_CHANNEL_ID);
  assert.equal(errorConfig.errorLogChannelId, null);

  const bothDisabled = { ...baseEnv };
  delete bothDisabled.GEOFENCE_LOG_CHANNEL_ID;
  delete bothDisabled.GEOFENCE_ERROR_LOG_CHANNEL_ID;
  const noLogConfig = loadBotConfig(bothDisabled);

  assert.equal(noLogConfig.logChannelId, null);
  assert.equal(noLogConfig.errorLogChannelId, null);
});

test("loadBotConfig supports more than ten contiguous servers", () => {
  const env = { ...baseEnv };
  for (let number = 3; number <= 11; number += 1) {
    env[`HLL_HOST_${number}`] = `192.0.2.${number + 9}`;
    env[`HLL_PORT_${number}`] = `${7778 + number}`;
    env[`HLL_PASSWORD_${number}`] = `secret-${number}`;
    env[`GEOFENCE_CHANNEL_S${number}_ID`] = `${123456789012345680n + BigInt(number)}`;
  }

  const config = loadBotConfig(env);

  assert.equal(config.servers.length, 11);
  assert.equal(config.servers[10].number, 11);
  assert.equal(config.servers[10].channelId, "123456789012345691");
});

for (const fixture of [
  {
    name: "a server numbering gap",
    update(env) {
      delete env.HLL_HOST_2;
      delete env.HLL_PORT_2;
      delete env.HLL_PASSWORD_2;
      delete env.HLL_SERVER_LABEL_2;
      delete env.GEOFENCE_CHANNEL_S2_ID;
      env.HLL_HOST_3 = "192.0.2.12";
      env.HLL_PORT_3 = "7799";
      env.HLL_PASSWORD_3 = "third-secret";
      env.GEOFENCE_CHANNEL_S3_ID = "123456789012345683";
    },
    message: "server numbering must be contiguous"
  },
  {
    name: "an incomplete second server",
    update(env) { delete env.HLL_PASSWORD_2; },
    message: "HLL_PASSWORD_2"
  },
  {
    name: "a missing second control channel",
    update(env) { delete env.GEOFENCE_CHANNEL_S2_ID; },
    message: "GEOFENCE_CHANNEL_S2_ID"
  },
  {
    name: "a duplicate Discord channel",
    update(env) { env.GEOFENCE_LOG_CHANNEL_ID = env.GEOFENCE_CHANNEL_S1_ID; },
    message: "Discord channel IDs must be distinct"
  },
  {
    name: "an invalid Discord ID",
    update(env) { env.DISCORD_GUILD_ID = "not-an-id"; },
    message: "DISCORD_GUILD_ID"
  },
  {
    name: "an unsupported language",
    update(env) { env.GEOFENCE_LANGUAGE = "fr"; },
    message: "GEOFENCE_LANGUAGE must be en or de"
  },
  {
    name: "a profile limit above 100",
    update(env) { env.GEOFENCE_MIDCAP_MAX_PLAYERS = "101"; },
    message: "GEOFENCE_MIDCAP_MAX_PLAYERS"
  },
  {
    name: "a multiline token",
    update(env) { env.DISCORD_TOKEN = "first\nsecond"; },
    message: "DISCORD_TOKEN must not contain line breaks"
  }
]) {
  test(`loadBotConfig rejects ${fixture.name}`, () => {
    const env = { ...baseEnv };
    fixture.update(env);

    assert.throws(
      () => loadBotConfig(env),
      (error) => error instanceof ConfigurationError
        && error.message.includes(fixture.message)
    );
  });
}
