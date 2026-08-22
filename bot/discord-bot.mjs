import {
  Events,
  MessageFlags,
  PermissionFlagsBits
} from "discord.js";

import { buildWorkerLogPayload, parseWorkerLine } from "./log-parser.mjs";
import {
  buildPanel,
  isInteractionAuthorized,
  isManagedPanelMessage,
  parseControlId
} from "./ui.mjs";

const REQUIRED_CHANNEL_PERMISSIONS = [
  PermissionFlagsBits.ViewChannel,
  PermissionFlagsBits.SendMessages,
  PermissionFlagsBits.EmbedLinks,
  PermissionFlagsBits.ReadMessageHistory
];

function publicError(error) {
  return `${error?.publicMessage ?? error?.message ?? "Unknown error"}`.slice(0, 1_500);
}

export class DiscordGeofenceBot {
  constructor({ config, supervisor, client, panelStore, health, logger = console }) {
    this.config = config;
    this.supervisor = supervisor;
    this.client = client;
    this.panelStore = panelStore;
    this.health = health;
    this.logger = logger;
    this.channels = new Map();
    this.panelMessages = new Map();
    this.panelIds = {};
    this.liveStatuses = new Map();
    this.busyServers = new Set();
    this.refreshTimer = null;
    this.ready = false;

    supervisor.on("worker-line", (record) => {
      this.handleWorkerLine(record).catch((error) => {
        this.logger.error("worker-line-handler-failed", { error: error.message });
      });
    });
    supervisor.on("worker-error", (record) => {
      this.handleWorkerError(record).catch((error) => {
        this.logger.error("worker-error-handler-failed", { error: error.message });
      });
    });
    supervisor.on("worker-status", (status) => {
      if (this.ready) {
        this.upsertPanel(status.serverNumber).catch((error) => {
          this.logger.error("panel-status-update-failed", { error: error.message });
        });
      }
    });
  }

  server(serverNumber) {
    return this.config.servers.find((candidate) => candidate.number === serverNumber) ?? null;
  }

  async resolveChannel(channelId, key) {
    const channel = await this.client.channels.fetch(channelId);
    if (!channel?.isTextBased?.() || channel.guildId !== this.config.guildId) {
      throw new Error(`${key} is not a text channel in DISCORD_GUILD_ID.`);
    }
    const permissions = channel.permissionsFor?.(this.client.user);
    if (!permissions?.has(REQUIRED_CHANNEL_PERMISSIONS)) {
      throw new Error(`${key} is missing required Discord channel permissions.`);
    }
    return channel;
  }

  async loadChannels() {
    for (const server of this.config.servers) {
      this.channels.set(
        server.number,
        await this.resolveChannel(server.channelId, `GEOFENCE_CHANNEL_S${server.number}_ID`)
      );
    }
    this.logChannel = this.config.logChannelId
      ? await this.resolveChannel(this.config.logChannelId, "GEOFENCE_LOG_CHANNEL_ID")
      : null;
    this.errorLogChannel = this.config.errorLogChannelId
      ? await this.resolveChannel(
        this.config.errorLogChannelId,
        "GEOFENCE_ERROR_LOG_CHANNEL_ID"
      )
      : null;
  }

  async discoverPanel(server) {
    const channel = this.channels.get(server.number);
    const storedId = this.panelIds[`${server.number}`];
    const managed = new Map();
    if (storedId) {
      try {
        const message = await channel.messages.fetch(storedId);
        if (isManagedPanelMessage(message, server.number, this.client.user.id)) {
          managed.set(message.id, message);
        }
      } catch {
        // The stored message may have been deleted; discover or recreate it.
      }
    }

    let before;
    const seenCursors = new Set();
    while (true) {
      const page = await channel.messages.fetch({
        limit: 100,
        ...(before ? { before } : {})
      });
      const messages = [...page.values()];
      for (const message of messages) {
        if (isManagedPanelMessage(message, server.number, this.client.user.id)) {
          managed.set(message.id, message);
        }
      }
      if (messages.length < 100) break;
      const nextCursor = messages.at(-1)?.id;
      if (!nextCursor || seenCursors.has(nextCursor)) break;
      seenCursors.add(nextCursor);
      before = nextCursor;
    }

    const selected = managed.get(storedId) ?? managed.values().next().value ?? null;
    for (const message of managed.values()) {
      if (message.id !== selected?.id) {
        await message.delete().catch((error) => {
          this.logger.warn("duplicate-panel-delete-failed", {
            serverNumber: server.number,
            messageId: message.id,
            error: error.message
          });
        });
      }
    }
    return selected;
  }

  async upsertPanel(serverNumber, busy = this.busyServers.has(serverNumber)) {
    const server = this.server(serverNumber);
    if (!server) return;
    const channel = this.channels.get(serverNumber);
    if (!channel) return;
    const payload = buildPanel({
      config: this.config,
      server,
      workerStatus: this.supervisor.status(serverNumber),
      liveStatus: this.liveStatuses.get(serverNumber) ?? null,
      busy
    });
    let message = this.panelMessages.get(serverNumber);
    if (!message) {
      message = await this.discoverPanel(server);
    }
    if (message) {
      await message.edit(payload);
    } else {
      message = await channel.send(payload);
    }
    this.panelMessages.set(serverNumber, message);
    this.panelIds[`${serverNumber}`] = message.id;
    await this.panelStore.save(this.panelIds);
  }

  async refreshPanels() {
    for (const server of this.config.servers) {
      await this.upsertPanel(server.number);
    }
  }

  scheduleRefresh() {
    clearTimeout(this.refreshTimer);
    this.refreshTimer = setTimeout(async () => {
      try {
        await this.refreshPanels();
        await this.health.ready("periodic-refresh");
      } catch (error) {
        this.logger.error("periodic-refresh-failed", { error: error.message });
        await this.health.failed(error.message).catch(() => {});
      } finally {
        this.scheduleRefresh();
      }
    }, this.config.refreshIntervalMs);
    this.refreshTimer.unref?.();
  }

  async onReady() {
    await this.loadChannels();
    this.panelIds = await this.panelStore.load();
    await this.supervisor.restore();
    this.ready = true;
    await this.refreshPanels();
    await this.health.ready("discord-ready");
    this.scheduleRefresh();
    this.logger.info("discord-bot-ready", {
      botUserId: this.client.user.id,
      servers: this.config.servers.map((server) => server.number)
    });
  }

  async handleWorkerLine(record) {
    const event = parseWorkerLine(record);
    if (!event) return;
    if (event.kind === "status") {
      this.liveStatuses.set(event.serverNumber, event.status);
      if (this.ready) await this.upsertPanel(event.serverNumber);
      return;
    }
    const server = this.server(event.serverNumber);
    if (!server) return;
    const profile = this.supervisor.status(event.serverNumber).profile ?? "none";
    const payload = buildWorkerLogPayload(event, {
      language: this.config.language,
      serverLabel: server.label,
      profile,
      punishAfterSeconds: this.config.punishAfterSeconds
    });
    const channel = event.kind === "error" ? this.errorLogChannel : this.logChannel;
    if (channel) await channel.send(payload);
  }

  async handleWorkerError(record) {
    const timestamp = new Date().toISOString();
    await this.handleWorkerLine({
      serverNumber: record.serverNumber,
      source: "supervisor",
      line: `time=${timestamp} level=ERROR msg=worker-exit error=${JSON.stringify(record.message)}`
    });
  }

  async handleInteraction(interaction) {
    if (!interaction.isButton?.()) return;
    const request = parseControlId(interaction.customId);
    const server = request ? this.server(request.serverNumber) : null;
    if (!request || !server) return;
    const german = this.config.language === "de";
    if (!isInteractionAuthorized(interaction, this.config, server)) {
      await interaction.reply({
        content: german
          ? "Du bist für diese Geofence-Steuerung nicht berechtigt."
          : "You are not authorized to use this geofence control.",
        ephemeral: true
      });
      return;
    }
    if (this.busyServers.has(server.number)) {
      await interaction.reply({
        content: german
          ? "Für diesen Server läuft bereits eine Geofence-Aktion."
          : "A geofence action is already running for this server.",
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    await interaction.deferReply({ flags: MessageFlags.Ephemeral });
    this.busyServers.add(server.number);
    try {
      await this.upsertPanel(server.number, true);
      if (request.action === "start") {
        await this.supervisor.start(server.number, request.profile);
      } else if (request.action === "stop") {
        await this.supervisor.stop(server.number);
      } else if (request.action === "refresh") {
        await this.supervisor.refresh(server.number);
      }
      await interaction.editReply({
        content: german
          ? `Geofence-Aktion für Server ${server.number} wurde ausgeführt.`
          : `Geofence action for Server ${server.number} completed.`
      });
      await this.health.ready("discord-action-completed");
    } catch (error) {
      this.logger.error("discord-action-failed", {
        serverNumber: server.number,
        action: request.action,
        profile: request.profile,
        actorId: interaction.user?.id,
        error: error.message
      });
      await interaction.editReply({
        content: german
          ? `Geofence-Aktion fehlgeschlagen: ${publicError(error)}`
          : `Geofence action failed: ${publicError(error)}`
      }).catch(() => {});
      await this.health.failed(error.message).catch(() => {});
    } finally {
      this.busyServers.delete(server.number);
      await this.upsertPanel(server.number).catch(() => {});
    }
  }

  async run() {
    this.client.once(Events.ClientReady, () => {
      this.onReady().catch(async (error) => {
        this.logger.error("discord-startup-failed", { error: error.message });
        await this.health.failed(error.message).catch(() => {});
        await this.supervisor.shutdown().catch((shutdownError) => {
          this.logger.error("startup-worker-shutdown-failed", {
            error: shutdownError.message
          });
        });
        this.client.destroy();
        process.exitCode = 1;
      });
    });
    this.client.on(Events.InteractionCreate, (interaction) => {
      this.handleInteraction(interaction).catch((error) => {
        this.logger.error("discord-interaction-handler-failed", { error: error.message });
      });
    });
    await this.client.login(this.config.token);
  }

  async shutdown() {
    clearTimeout(this.refreshTimer);
    this.refreshTimer = null;
    this.ready = false;
    await this.supervisor.shutdown();
    this.client.destroy();
  }
}
