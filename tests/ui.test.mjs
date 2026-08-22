import assert from "node:assert/strict";
import test from "node:test";

import {
  buildPanel,
  isInteractionAuthorized,
  parseControlId
} from "../bot/ui.mjs";

const config = Object.freeze({
  guildId: "123456789012345678",
  language: "en",
  profileLimits: { midcap: 42, "midcap-advanced": 88, lastcap: 77 },
  allowedUserIds: ["123456789012345690"],
  allowedRoleIds: ["123456789012345691"]
});
const server = Object.freeze({
  number: 2,
  label: "Vietnam Two",
  channelId: "123456789012345680"
});

test("buildPanel exposes all profile actions with configured inclusive limits", () => {
  const payload = buildPanel({
    config,
    server,
    workerStatus: {
      serverNumber: 2,
      profile: "midcap",
      state: "running",
      pid: 123,
      lastError: null
    },
    liveStatus: {
      serverName: "Vietnam Two",
      map: "wdeva_warfare_day",
      mode: "Warfare",
      playerCount: 35,
      maxPlayerCount: 100,
      queueCount: 1,
      maxQueueCount: 6,
      refreshedAt: "2026-08-22T12:00:00.000Z"
    }
  });
  const embed = payload.embeds[0].toJSON();
  const buttons = payload.components.flatMap((row) => row.toJSON().components);

  assert.match(embed.title, /Vietnam Two/);
  assert.match(embed.fields.find((field) => field.name === "Current profile").value, /Midcap/);
  assert.match(embed.fields.find((field) => field.name === "Live server").value, /35 \/ 100/);
  assert.deepEqual(buttons.map((button) => button.custom_id), [
    "gf:v1:s2:start:midcap",
    "gf:v1:s2:start:midcap-advanced",
    "gf:v1:s2:start:lastcap",
    "gf:v1:s2:stop:all",
    "gf:v1:s2:refresh:all"
  ]);
  assert.deepEqual(buttons.slice(0, 3).map((button) => button.label), [
    "Midcap (≤42)",
    "Midcap Advanced (≤88)",
    "Lastcap (≤77)"
  ]);
  assert.deepEqual(payload.allowedMentions, { parse: [] });
});

test("buildPanel localizes the Discord UI in German", () => {
  const payload = buildPanel({
    config: { ...config, language: "de" },
    server,
    workerStatus: {
      serverNumber: 2,
      profile: null,
      state: "stopped",
      pid: null,
      lastError: null
    },
    liveStatus: null
  });
  const embed = payload.embeds[0].toJSON();
  const buttons = payload.components.flatMap((row) => row.toJSON().components);

  assert.equal(embed.fields[0].name, "Aktuelles Profil");
  assert.match(embed.fields[0].value, /gestoppt/i);
  assert.equal(buttons[3].label, "Geofence stoppen");
  assert.equal(buttons[4].label, "Status aktualisieren");
});

test("parseControlId accepts only bounded known Discord actions", () => {
  assert.deepEqual(parseControlId("gf:v1:s2:start:midcap-advanced"), {
    serverNumber: 2,
    action: "start",
    profile: "midcap-advanced"
  });
  assert.deepEqual(parseControlId("gf:v1:s10:stop:all"), {
    serverNumber: 10,
    action: "stop",
    profile: null
  });
  assert.deepEqual(parseControlId("gf:v1:s11:start:midcap"), {
    serverNumber: 11,
    action: "start",
    profile: "midcap"
  });
  assert.equal(parseControlId("gf:v1:s0:start:midcap"), null);
  assert.equal(parseControlId("gf:v1:s2:start:unknown"), null);
});

test("isInteractionAuthorized requires the configured guild, channel, and allowlist", () => {
  const interaction = {
    guildId: config.guildId,
    channelId: server.channelId,
    user: { id: "123456789012345699" },
    member: { roles: ["123456789012345691"] },
    inGuild: () => true
  };

  assert.equal(isInteractionAuthorized(interaction, config, server), true);
  assert.equal(isInteractionAuthorized(
    { ...interaction, channelId: "123456789012345700" },
    config,
    server
  ), false);
  assert.equal(isInteractionAuthorized(
    { ...interaction, member: { roles: [] } },
    config,
    server
  ), false);
  assert.equal(isInteractionAuthorized(
    { ...interaction, user: { id: "123456789012345690" }, member: { roles: [] } },
    config,
    server
  ), true);
});
