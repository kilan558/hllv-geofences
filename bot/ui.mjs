import {
  ActionRowBuilder,
  ButtonBuilder,
  ButtonStyle,
  EmbedBuilder
} from "discord.js";

const PROFILES = Object.freeze([
  Object.freeze({ id: "midcap", en: "Midcap", de: "Midcap" }),
  Object.freeze({ id: "midcap-advanced", en: "Midcap Advanced", de: "Midcap erweitert" }),
  Object.freeze({ id: "lastcap", en: "Lastcap", de: "Lastcap" })
]);

const MAP_NAMES = Object.freeze({
  wdeva: "Vạn Tường",
  wdevb: "Quảng Ngãi",
  wdevc: "Huế Outskirts",
  wdevd: "Đăk Tô Airfield",
  wdeve: "Cam Ranh Port",
  wdevf: "Thanh Hòa Bridge"
});

function profileLabel(profile, language) {
  const definition = PROFILES.find((candidate) => candidate.id === profile);
  return definition?.[language] ?? profile ?? (language === "de" ? "Gestoppt" : "Stopped");
}

function mapLabel(value) {
  const prefix = `${value ?? ""}`.trim().toLowerCase().slice(0, 5);
  return MAP_NAMES[prefix] ?? `${value ?? "Unknown"}`;
}

function workerStateLabel(status, language) {
  const labels = language === "de"
    ? { running: "🟢 Aktiv", stopped: "⚫ Gestoppt", failed: "🔴 Fehler", starting: "🟡 Startet" }
    : { running: "🟢 Active", stopped: "⚫ Stopped", failed: "🔴 Failed", starting: "🟡 Starting" };
  return labels[status.state] ?? `🔴 ${status.state}`;
}

export function buildPanel({ config, server, workerStatus, liveStatus = null, busy = false }) {
  const german = config.language === "de";
  const texts = german
    ? {
        current: "Aktuelles Profil",
        live: "Live-Server",
        worker: "Worker-Status",
        unavailable: "Noch keine RCON-Livedaten verfügbar",
        stop: "Geofence stoppen",
        refresh: "Status aktualisieren",
        description: "Eigenständige Steuerung für die sechs unterstützten HLL: Vietnam Warfare-Karten. Pro Server kann genau ein Profil aktiv sein."
      }
    : {
        current: "Current profile",
        live: "Live server",
        worker: "Worker status",
        unavailable: "No live RCON data available yet",
        stop: "Stop geofence",
        refresh: "Refresh status",
        description: "Standalone control for the six supported HLL: Vietnam Warfare maps. Exactly one profile can be active per server."
      };
  const activeProfile = workerStatus.profile
    ? `**${profileLabel(workerStatus.profile, config.language)}**`
    : `**${profileLabel(null, config.language)}**`;
  const liveValue = liveStatus
    ? `**${liveStatus.playerCount} / ${liveStatus.maxPlayerCount}** · ${mapLabel(liveStatus.map)} · ${liveStatus.mode}\n`
      + `${german ? "Warteschlange" : "Queue"}: ${liveStatus.queueCount} / ${liveStatus.maxQueueCount}`
    : texts.unavailable;

  const embed = new EmbedBuilder()
    .setTitle(`HLL: Vietnam Geofence · ${server.label}`)
    .setColor(workerStatus.state === "running" ? 0x4f8a3c : workerStatus.state === "failed" ? 0xc0392b : 0x5b6470)
    .setDescription(texts.description)
    .addFields(
      { name: texts.current, value: activeProfile, inline: true },
      { name: texts.worker, value: workerStateLabel(workerStatus, config.language), inline: true },
      { name: texts.live, value: liveValue, inline: false }
    )
    .setFooter({ text: german ? "Spielerlimits sind inklusive" : "Player limits are inclusive" })
    .setTimestamp();
  if (workerStatus.lastError) {
    embed.addFields({
      name: german ? "Letzter Fehler" : "Last error",
      value: `${workerStatus.lastError}`.slice(0, 1_024),
      inline: false
    });
  }

  const profileButtons = PROFILES.map((profile) => new ButtonBuilder()
    .setCustomId(`gf:v1:s${server.number}:start:${profile.id}`)
    .setLabel(`${profileLabel(profile.id, config.language)} (≤${config.profileLimits[profile.id]})`)
    .setStyle(ButtonStyle.Success)
    .setDisabled(busy || (workerStatus.state === "running" && workerStatus.profile === profile.id)));
  const controls = new ActionRowBuilder().addComponents(
    new ButtonBuilder()
      .setCustomId(`gf:v1:s${server.number}:stop:all`)
      .setLabel(texts.stop)
      .setStyle(ButtonStyle.Danger)
      .setDisabled(busy || workerStatus.state === "stopped"),
    new ButtonBuilder()
      .setCustomId(`gf:v1:s${server.number}:refresh:all`)
      .setLabel(texts.refresh)
      .setStyle(ButtonStyle.Secondary)
      .setDisabled(busy || workerStatus.state !== "running" || !liveStatus)
  );
  return {
    embeds: [embed],
    components: [new ActionRowBuilder().addComponents(profileButtons), controls],
    allowedMentions: { parse: [] }
  };
}

export function parseControlId(customId) {
  const match = `${customId ?? ""}`.match(
    /^gf:v1:s([1-9]\d*):(start|stop|refresh):(midcap|midcap-advanced|lastcap|all)$/
  );
  if (!match) return null;
  const action = match[2];
  if (action === "start" && match[3] === "all") return null;
  if (action !== "start" && match[3] !== "all") return null;
  const serverNumber = Number(match[1]);
  if (!Number.isSafeInteger(serverNumber)) return null;
  return {
    serverNumber,
    action,
    profile: action === "start" ? match[3] : null
  };
}

function roleIds(interaction) {
  if (Array.isArray(interaction.member?.roles)) return new Set(interaction.member.roles);
  if (interaction.member?.roles?.cache?.keys) {
    return new Set(interaction.member.roles.cache.keys());
  }
  return new Set();
}

export function isInteractionAuthorized(interaction, config, server) {
  if (
    !interaction.inGuild?.()
    || interaction.guildId !== config.guildId
    || interaction.channelId !== server.channelId
  ) {
    return false;
  }
  if (config.allowedUserIds.length === 0 && config.allowedRoleIds.length === 0) {
    return true;
  }
  if (config.allowedUserIds.includes(interaction.user?.id)) return true;
  const roles = roleIds(interaction);
  return config.allowedRoleIds.some((roleId) => roles.has(roleId));
}

export function isManagedPanelMessage(message, serverNumber, botUserId) {
  if (message?.author?.id !== botUserId) return false;
  return message.components?.some((row) => row.components?.some((component) =>
    `${component.customId ?? component.custom_id ?? ""}`.startsWith(`gf:v1:s${serverNumber}:`)
  )) ?? false;
}
