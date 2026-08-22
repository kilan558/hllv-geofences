import assert from "node:assert/strict";
import test from "node:test";

import {
  buildWorkerLogPayload,
  parseWorkerLine
} from "../bot/log-parser.mjs";

test("parseWorkerLine extracts live session data for the Discord panel", () => {
  const event = parseWorkerLine({
    serverNumber: 2,
    source: "stdout",
    line: 'time=2026-08-22T12:00:00.000Z level=INFO msg=session-status server_name="Vietnam Two" map=wdevf_warfare_day mode=Warfare player_count=42 max_player_count=100 queue_count=2 max_queue_count=6'
  });

  assert.deepEqual(event, {
    kind: "status",
    serverNumber: 2,
    timestamp: "2026-08-22T12:00:00.000Z",
    status: {
      serverName: "Vietnam Two",
      map: "wdevf_warfare_day",
      mode: "Warfare",
      playerCount: 42,
      maxPlayerCount: 100,
      queueCount: 2,
      maxQueueCount: 6,
      refreshedAt: "2026-08-22T12:00:00.000Z"
    }
  });
});

test("parseWorkerLine classifies warning, punish, and errors", () => {
  const warning = parseWorkerLine({
    serverNumber: 1,
    source: "stdout",
    line: 'time=2026-08-22T12:00:01.000Z level=INFO msg=player-outside-fence player="Test Player" player_id=steam-1 grid="J8 Numpad 2" map=wdeva_warfare_day mode=Warfare'
  });
  const punish = parseWorkerLine({
    serverNumber: 1,
    source: "stdout",
    line: 'time=2026-08-22T12:00:11.000Z level=INFO msg=punish-player player="Test Player" player_id=steam-1 grid="J8 Numpad 2" map=wdeva_warfare_day mode=Warfare'
  });
  const error = parseWorkerLine({
    serverNumber: 1,
    source: "stderr",
    line: 'time=2026-08-22T12:00:12.000Z level=ERROR msg=poll-session error="connection timeout"'
  });

  assert.equal(warning.kind, "warning");
  assert.equal(warning.fields.player, "Test Player");
  assert.equal(punish.kind, "punish");
  assert.equal(error.kind, "error");
  assert.equal(error.fields.error, "connection timeout");
});

test("buildWorkerLogPayload disables mentions and localizes punish details", () => {
  const event = parseWorkerLine({
    serverNumber: 1,
    source: "stdout",
    line: 'time=2026-08-22T12:00:11.000Z level=INFO msg=punish-player player="@everyone" player_id=steam-1 grid=A1 map=wdeva_warfare_day mode=Warfare'
  });

  const payload = buildWorkerLogPayload(event, {
    language: "de",
    serverLabel: "Vietnam One",
    profile: "midcap",
    punishAfterSeconds: 17
  });
  const json = payload.embeds[0].toJSON();

  assert.deepEqual(payload.allowedMentions, { parse: [] });
  assert.match(json.title, /Geofence-Bestrafung/);
  assert.match(json.description, /17 Sekunden/);
  assert.equal(json.fields.find((field) => field.name === "Spieler").value, "@everyone");
});

test("parseWorkerLine ignores unrelated informational records", () => {
  assert.equal(parseWorkerLine({
    serverNumber: 1,
    source: "stdout",
    line: "time=2026-08-22T12:00:00.000Z level=INFO msg=start"
  }), null);
});

test("parseWorkerLine accepts configured server numbers above ten", () => {
  const event = parseWorkerLine({
    serverNumber: 11,
    source: "stdout",
    line: "time=2026-08-22T12:00:00.000Z level=INFO msg=session-status map=wdeva_warfare_day mode=Warfare player_count=11 max_player_count=100 queue_count=0 max_queue_count=6"
  });

  assert.equal(event.serverNumber, 11);
  assert.equal(event.status.playerCount, 11);
});
