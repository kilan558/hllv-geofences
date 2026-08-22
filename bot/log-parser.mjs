import { EmbedBuilder } from "discord.js";

const RELEVANT_MESSAGES = new Map([
  ["player-outside-fence", "warning"],
  ["punish-player", "punish"]
]);

function parseLogfmt(input) {
  const fields = {};
  let index = 0;
  while (index < input.length) {
    while (input[index] === " ") index += 1;
    const keyStart = index;
    while (index < input.length && input[index] !== "=" && input[index] !== " ") {
      index += 1;
    }
    if (input[index] !== "=") {
      while (index < input.length && input[index] !== " ") index += 1;
      continue;
    }
    const key = input.slice(keyStart, index);
    index += 1;
    let value = "";
    if (input[index] === '"') {
      const start = index;
      index += 1;
      let escaped = false;
      while (index < input.length) {
        const character = input[index];
        index += 1;
        if (escaped) {
          escaped = false;
        } else if (character === "\\") {
          escaped = true;
        } else if (character === '"') {
          break;
        }
      }
      try {
        value = JSON.parse(input.slice(start, index));
      } catch {
        value = input.slice(start + 1, Math.max(start + 1, index - 1));
      }
    } else {
      const start = index;
      while (index < input.length && input[index] !== " ") index += 1;
      value = input.slice(start, index);
    }
    if (key) fields[key] = value;
  }
  return fields;
}

function safeInteger(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed >= 0 ? parsed : fallback;
}

function timestamp(fields) {
  const value = `${fields.time ?? ""}`;
  return Number.isFinite(Date.parse(value)) ? value : new Date().toISOString();
}

export function parseWorkerLine({ serverNumber, source, line }) {
  if (!Number.isSafeInteger(serverNumber) || serverNumber < 1) {
    return null;
  }
  const fields = parseLogfmt(`${line ?? ""}`);
  const level = `${fields.level ?? ""}`.toUpperCase();
  const eventTimestamp = timestamp(fields);
  if (level === "INFO" && fields.msg === "session-status") {
    return {
      kind: "status",
      serverNumber,
      timestamp: eventTimestamp,
      status: {
        serverName: `${fields.server_name ?? `Server ${serverNumber}`}`.slice(0, 100),
        map: `${fields.map ?? "unknown"}`.slice(0, 100),
        mode: `${fields.mode ?? "unknown"}`.slice(0, 32),
        playerCount: safeInteger(fields.player_count),
        maxPlayerCount: safeInteger(fields.max_player_count, 100),
        queueCount: safeInteger(fields.queue_count),
        maxQueueCount: safeInteger(fields.max_queue_count),
        refreshedAt: eventTimestamp
      }
    };
  }
  const kind = level === "ERROR" ? "error" : RELEVANT_MESSAGES.get(fields.msg);
  if (!kind) {
    return null;
  }
  return {
    kind,
    serverNumber,
    timestamp: eventTimestamp,
    source,
    fields,
    raw: `${line ?? ""}`.slice(0, 3_500)
  };
}

function clipped(value, fallback = "Unknown", maximum = 1_024) {
  const text = `${value ?? ""}`.trim() || fallback;
  return text.length <= maximum ? text : `${text.slice(0, maximum - 1)}…`;
}

function addField(fields, name, value, inline = true) {
  if (`${value ?? ""}`.trim()) {
    fields.push({ name, value: clipped(value), inline });
  }
}

export function buildWorkerLogPayload(event, context) {
  const german = context.language === "de";
  const definitions = german
    ? {
        warning: ["⚠️ Geofence-Warnung", 0xf1c40f, "Ein Spieler befindet sich außerhalb des freigegebenen Bereichs."],
        punish: ["⛔ Geofence-Bestrafung", 0xe67e22, `Der Spieler wurde nach ${context.punishAfterSeconds} Sekunden bestraft.`],
        error: ["❌ Geofence-Fehler", 0xc0392b, "Der Geofence-Worker hat einen Fehler gemeldet."]
      }
    : {
        warning: ["⚠️ Geofence warning", 0xf1c40f, "A player is outside the permitted play area."],
        punish: ["⛔ Geofence punishment", 0xe67e22, `The player was punished after ${context.punishAfterSeconds} seconds.`],
        error: ["❌ Geofence error", 0xc0392b, "The geofence worker reported an error."]
      };
  const [title, color, description] = definitions[event.kind];
  const names = german
    ? { player: "Spieler", playerId: "Spieler-ID", grid: "Grid", map: "Karte", mode: "Modus", profile: "Profil", error: "Fehler", raw: "Rohlog" }
    : { player: "Player", playerId: "Player ID", grid: "Grid", map: "Map", mode: "Mode", profile: "Profile", error: "Error", raw: "Raw log" };
  const fields = [];
  addField(fields, names.player, event.fields.player);
  addField(fields, names.playerId, event.fields.player_id);
  addField(fields, names.grid, event.fields.grid);
  addField(fields, names.map, event.fields.map);
  addField(fields, names.mode, event.fields.mode);
  addField(fields, names.profile, context.profile);
  if (event.kind === "error") {
    addField(fields, names.error, event.fields.error ?? event.fields.msg, false);
    addField(fields, names.raw, event.raw, false);
  }
  const embed = new EmbedBuilder()
    .setTitle(`${title} · ${context.serverLabel}`)
    .setColor(color)
    .setDescription(description)
    .setTimestamp(new Date(event.timestamp));
  if (fields.length > 0) embed.addFields(fields);
  return { embeds: [embed], allowedMentions: { parse: [] } };
}
