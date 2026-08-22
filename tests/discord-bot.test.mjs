import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import test from "node:test";

import { DiscordGeofenceBot } from "../bot/discord-bot.mjs";

class FakeChannel {
  constructor(id, guildId) {
    this.id = id;
    this.guildId = guildId;
    this.sent = [];
    this.messagesById = new Map();
    this.historyPages = null;
    this.historyPageIndex = 0;
    this.messages = {
      fetch: async (argument) => {
        if (typeof argument === "string") {
          const message = this.messagesById.get(argument);
          if (!message) throw new Error("unknown message");
          return message;
        }
        if (this.historyPages) {
          return this.historyPages[this.historyPageIndex++] ?? new Map();
        }
        return new Map([...this.messagesById.entries()]);
      }
    };
  }

  isTextBased() { return true; }
  permissionsFor() { return { has: () => true }; }

  async send(payload) {
    this.sent.push(payload);
    const message = {
      id: `message-${this.sent.length}`,
      author: { id: "bot-user" },
      components: payload.components?.map((row) => row.toJSON()) ?? [],
      async edit(nextPayload) {
        this.components = nextPayload.components?.map((row) => row.toJSON()) ?? [];
        this.lastPayload = nextPayload;
        return this;
      }
    };
    this.messagesById.set(message.id, message);
    return message;
  }
}

class FakeSupervisor extends EventEmitter {
  constructor() {
    super();
    this.current = {
      serverNumber: 1,
      profile: "midcap",
      state: "running",
      pid: 123,
      lastError: null
    };
    this.actions = [];
  }

  async restore() {}
  status() { return { ...this.current }; }
  async start(serverNumber, profile) {
    this.actions.push({ action: "start", serverNumber, profile });
    this.current = { ...this.current, profile, state: "running" };
  }
  async stop(serverNumber) {
    this.actions.push({ action: "stop", serverNumber });
    this.current = { ...this.current, profile: null, state: "stopped", pid: null };
  }
  async refresh(serverNumber) {
    this.actions.push({ action: "refresh", serverNumber });
    return { serverNumber, line: "level=INFO msg=session-status player_count=36" };
  }
  async shutdown() {}
}

function fixture() {
  const config = {
    guildId: "123456789012345678",
    language: "en",
    profileLimits: { midcap: 42, "midcap-advanced": 88, lastcap: 77 },
    punishAfterSeconds: 17,
    allowedUserIds: ["123456789012345690"],
    allowedRoleIds: [],
    refreshIntervalMs: 60_000,
    logChannelId: "123456789012345681",
    errorLogChannelId: "123456789012345682",
    servers: [{
      number: 1,
      label: "Vietnam One",
      channelId: "123456789012345679"
    }]
  };
  const channels = new Map([
    [config.servers[0].channelId, new FakeChannel(config.servers[0].channelId, config.guildId)],
    [config.logChannelId, new FakeChannel(config.logChannelId, config.guildId)],
    [config.errorLogChannelId, new FakeChannel(config.errorLogChannelId, config.guildId)]
  ]);
  const client = {
    user: { id: "bot-user" },
    channels: { fetch: async (id) => channels.get(id) },
    destroy() {}
  };
  const panelIds = {};
  const panelStore = {
    async load() { return { ...panelIds }; },
    async save(value) { Object.assign(panelIds, value); }
  };
  const health = {
    async ready() {},
    async failed() {}
  };
  const supervisor = new FakeSupervisor();
  const bot = new DiscordGeofenceBot({
    config,
    supervisor,
    client,
    panelStore,
    health,
    logger: { info() {}, warn() {}, error() {} }
  });
  return { bot, channels, config, supervisor };
}

test("DiscordGeofenceBot creates a persistent panel and routes direct worker logs", async (t) => {
  const { bot, channels, config } = fixture();
  t.after(() => bot.shutdown());
  await bot.onReady();

  const controlChannel = channels.get(config.servers[0].channelId);
  assert.equal(controlChannel.sent.length, 1);

  await bot.handleWorkerLine({
    serverNumber: 1,
    source: "stdout",
    line: 'time=2026-08-22T12:00:00.000Z level=INFO msg=session-status server_name="Vietnam One" map=wdeva_warfare_day mode=Warfare player_count=35 max_player_count=100 queue_count=1 max_queue_count=6'
  });
  await bot.handleWorkerLine({
    serverNumber: 1,
    source: "stdout",
    line: 'time=2026-08-22T12:00:01.000Z level=INFO msg=player-outside-fence player="Test" player_id=steam-1 grid=A1 map=wdeva_warfare_day mode=Warfare'
  });
  await bot.handleWorkerLine({
    serverNumber: 1,
    source: "stderr",
    line: 'time=2026-08-22T12:00:02.000Z level=ERROR msg=poll-session error=timeout'
  });

  assert.equal(bot.liveStatuses.get(1).playerCount, 35);
  assert.equal(channels.get(config.logChannelId).sent.length, 1);
  assert.equal(channels.get(config.errorLogChannelId).sent.length, 1);
  assert.deepEqual(channels.get(config.logChannelId).sent[0].allowedMentions, { parse: [] });
});

test("DiscordGeofenceBot starts and discards worker log events when log channels are disabled", async (t) => {
  const { bot, channels, config } = fixture();
  t.after(() => bot.shutdown());
  config.logChannelId = null;
  config.errorLogChannelId = null;
  channels.delete("123456789012345681");
  channels.delete("123456789012345682");

  await bot.onReady();
  await bot.handleWorkerLine({
    serverNumber: 1,
    source: "stdout",
    line: 'time=2026-08-22T12:00:01.000Z level=INFO msg=player-outside-fence player="Test" player_id=steam-1 grid=A1 map=wdeva_warfare_day mode=Warfare'
  });
  await bot.handleWorkerLine({
    serverNumber: 1,
    source: "stderr",
    line: "time=2026-08-22T12:00:02.000Z level=ERROR msg=poll-session error=timeout"
  });

  assert.equal(channels.get(config.servers[0].channelId).sent.length, 1);
});

test("DiscordGeofenceBot paginates channel history and removes duplicate owned panels", async (t) => {
  const { bot, channels, config } = fixture();
  t.after(() => bot.shutdown());
  const controlChannel = channels.get(config.servers[0].channelId);
  const firstPage = new Map(Array.from({ length: 100 }, (_, index) => [
    `${900000000000000000n - BigInt(index)}`,
    { id: `${900000000000000000n - BigInt(index)}`, author: { id: "someone-else" }, components: [] }
  ]));
  const deleted = [];
  const managedMessage = (id) => ({
    id,
    author: { id: "bot-user" },
    components: [{ components: [{ custom_id: "gf:v1:s1:start:midcap" }] }],
    async edit(payload) { this.lastPayload = payload; return this; },
    async delete() { deleted.push(id); }
  });
  const keep = managedMessage("800000000000000001");
  const duplicate = managedMessage("800000000000000000");
  controlChannel.historyPages = [
    firstPage,
    new Map([[keep.id, keep], [duplicate.id, duplicate]]),
    new Map()
  ];

  await bot.onReady();

  assert.equal(controlChannel.sent.length, 0);
  assert.equal(bot.panelMessages.get(1).id, keep.id);
  assert.deepEqual(deleted, [duplicate.id]);
});

test("DiscordGeofenceBot authorizes and serializes a profile button action", async (t) => {
  const { bot, config, supervisor } = fixture();
  t.after(() => bot.shutdown());
  await bot.onReady();
  const replies = [];
  const interaction = {
    customId: "gf:v1:s1:start:lastcap",
    guildId: config.guildId,
    channelId: config.servers[0].channelId,
    user: { id: "123456789012345690" },
    member: { roles: [] },
    inGuild: () => true,
    isButton: () => true,
    async deferReply(payload) { replies.push({ type: "defer", payload }); },
    async editReply(payload) { replies.push({ type: "edit", payload }); }
  };

  await bot.handleInteraction(interaction);

  assert.deepEqual(supervisor.actions, [
    { action: "start", serverNumber: 1, profile: "lastcap" }
  ]);
  assert.match(replies.at(-1).payload.content, /Server 1/);
});

test("DiscordGeofenceBot rejects button use outside the configured allowlist", async (t) => {
  const { bot, config, supervisor } = fixture();
  t.after(() => bot.shutdown());
  await bot.onReady();
  const replies = [];
  const interaction = {
    customId: "gf:v1:s1:stop:all",
    guildId: config.guildId,
    channelId: config.servers[0].channelId,
    user: { id: "123456789012345699" },
    member: { roles: [] },
    inGuild: () => true,
    isButton: () => true,
    async reply(payload) { replies.push(payload); }
  };

  await bot.handleInteraction(interaction);

  assert.deepEqual(supervisor.actions, []);
  assert.equal(replies[0].ephemeral, true);
  assert.match(replies[0].content, /not authorized/i);
});

test("DiscordGeofenceBot refresh button requests immediate RCON status", async (t) => {
  const { bot, config, supervisor } = fixture();
  t.after(() => bot.shutdown());
  await bot.onReady();
  const interaction = {
    customId: "gf:v1:s1:refresh:all",
    guildId: config.guildId,
    channelId: config.servers[0].channelId,
    user: { id: "123456789012345690" },
    member: { roles: [] },
    inGuild: () => true,
    isButton: () => true,
    async deferReply() {},
    async editReply() {}
  };

  await bot.handleInteraction(interaction);

  assert.deepEqual(supervisor.actions, [{ action: "refresh", serverNumber: 1 }]);
});

test("DiscordGeofenceBot shuts down workers when Discord startup fails", async () => {
  const { bot, channels, config, supervisor } = fixture();
  let workersShutDown = false;
  let clientDestroyed = false;
  supervisor.restore = async () => { throw new Error("restore failed"); };
  supervisor.shutdown = async () => { workersShutDown = true; };

  const runtimeClient = new EventEmitter();
  runtimeClient.user = { id: "bot-user" };
  runtimeClient.channels = { fetch: async (id) => channels.get(id) };
  runtimeClient.destroy = () => { clientDestroyed = true; };
  runtimeClient.login = async () => {
    queueMicrotask(() => runtimeClient.emit("clientReady"));
  };
  bot.client = runtimeClient;

  await bot.run();
  await new Promise((resolve) => setTimeout(resolve, 20));

  assert.equal(workersShutDown, true);
  assert.equal(clientDestroyed, true);
  assert.equal(process.exitCode, 1);
  process.exitCode = 0;
  void config;
});
