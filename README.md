# HLL: Vietnam Geofences

A standalone, Discord-controlled Vietnam-only geofence service for Hell Let
Loose: Vietnam servers. It connects directly to native RCON, warns and punishes
players outside the configured play area, and provides one independent Discord
control panel per configured game server.

The public version includes its own Discord supervisor and has no dependency on
CRCON, Master Cache, an automation stack, another local project, or a Docker
socket. All operator settings live in one private `.env`. Per-server policies
are generated at runtime and never contain server addresses or passwords.

This is an unofficial community project and is not affiliated with or endorsed
by the game developer or publisher.

## Supported scope

Only Warfare on the following maps is accepted:

| Map | RCON prefix |
|---|---|
| Vạn Tường | `wdeva` |
| Quảng Ngãi | `wdevb` |
| Huế Outskirts | `wdevc` |
| Đăk Tô Airfield | `wdevd` |
| Cam Ranh Port | `wdeve` |
| Thanh Hòa Bridge | `wdevf` |

Configuration validation rejects Classic-HLL maps, unknown maps, fences
without explicit map conditions, and modes other than Warfare. When the live
server runs an unsupported map or mode, no fence matches and no player is
warned or punished.

## Quick start

Requirements:

- Docker with Compose v2 for the recommended complete setup;
- Go 1.26.6, Bash 4+, and Node.js 18.18+ only for local development.

Create the private environment file:

```bash
cp .env.example .env
chmod 0600 .env
```

Configure Discord, the connection, language, profile, and player limit once:

```dotenv
GEOFENCE_LANGUAGE=en
GEOFENCE_PROFILE=midcap
GEOFENCE_MIDCAP_MAX_PLAYERS=60
GEOFENCE_PUNISH_AFTER_SECONDS=10

DISCORD_TOKEN=replace-with-discord-bot-token
DISCORD_GUILD_ID=123456789012345678
GEOFENCE_CHANNEL_S1_ID=123456789012345681

# Optional Discord event forwarding:
# GEOFENCE_LOG_CHANNEL_ID=123456789012345679
# GEOFENCE_ERROR_LOG_CHANNEL_ID=123456789012345680

HLL_SERVER_LABEL="HLL Vietnam Server 1"
HLL_HOST=192.0.2.10
HLL_PORT=7779
HLL_PASSWORD=replace-with-rcon-password
```

Create the persistent state directory and start the complete service:

```bash
mkdir -p bot-data
sudo chown 1000:1000 bot-data
docker compose up -d --build
docker compose logs -f geofences
```

The ownership command matches the `GEOFENCE_UID=1000` and
`GEOFENCE_GID=1000` defaults in `.env.example`. If you change those values,
use the same numeric IDs with `chown` before starting the container.

On first launch, `GEOFENCE_PROFILE` starts for every configured server. Each
server can then be switched or stopped independently through its Discord
panel. The selected state survives container restarts in `bot-data/`.

## Discord setup

Create a Discord application and bot in the Discord Developer Portal. The bot
uses only the `Guilds` gateway intent; Message Content and Guild Members are
not required. Invite it to the configured guild and grant these permissions in
every configured control channel and optional log channel:

- View Channel;
- Send Messages;
- Embed Links;
- Read Message History.

Create one distinct control channel per HLL server. The event-log and error-log
channels are optional and can be enabled independently. The bot maintains
exactly one panel message in each control channel; it never clears unrelated
channel messages.

Panel actions are:

- `Midcap`;
- `Midcap Advanced`;
- `Lastcap`;
- `Stop geofence`;
- `Refresh status`.

The panel shows the active worker/profile and live map, mode, players, and
queue directly from RCON. When configured, warnings and punishments go to
`GEOFENCE_LOG_CHANNEL_ID`, while worker failures go to
`GEOFENCE_ERROR_LOG_CHANNEL_ID`. Leaving either value empty disables that type
of Discord forwarding. Discord mentions are disabled for all relayed game
data.

If both `ALLOWED_USER_IDS` and `ALLOWED_ROLE_IDS` are empty, anyone with access
to the control channel can use its buttons. Set one or both comma-separated
allowlists for an additional application-level restriction. On a public or
shared Discord server, configure an allowlist or restrict the control channel
to trusted server administrators.

## Environment configuration

All operator settings are read from `.env`. Server credentials and runtime
settings are never copied into `config.yml`.

| Variable | Default | Purpose |
|---|---:|---|
| `GEOFENCE_LANGUAGE` | `en` | Default messages: `en` or `de` |
| `GEOFENCE_PROFILE` | `midcap` | `midcap`, `midcap-advanced`, or `lastcap` |
| `GEOFENCE_MIDCAP_MAX_PLAYERS` | `60` | Inclusive player maximum for `midcap` |
| `GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS` | `99` | Inclusive maximum for `midcap-advanced` |
| `GEOFENCE_LASTCAP_MAX_PLAYERS` | `90` | Inclusive maximum for `lastcap` |
| `GEOFENCE_PUNISH_AFTER_SECONDS` | `10` | Grace period, from 1 to 3600 seconds |
| `GEOFENCE_WARNING_MESSAGE` | language default | Optional single-line warning text |
| `GEOFENCE_PUNISH_MESSAGE` | language default | Optional single-line punishment reason |
| `GEOFENCE_SECTOR_SIZE` | `20000` | Positive world-to-grid sector size |
| `GEOFENCE_CENTER_OFFSET_X` | `0` | Finite X-axis grid offset |
| `GEOFENCE_CENTER_OFFSET_Y` | `0` | Finite Y-axis grid offset |
| `WHITELIST` | empty | Optional comma-separated player IDs |
| `COMMUNITY_NAME` | `HLL: Vietnam` | Name included in application logs |
| `DEBUG` | unset | Enable debug logging when present |
| `DISCORD_TOKEN` | required | Private Discord bot token |
| `DISCORD_GUILD_ID` | required | Guild containing every configured channel |
| `GEOFENCE_CHANNEL_S<N>_ID` | required per server | Independent control-panel channel |
| `GEOFENCE_LOG_CHANNEL_ID` | empty | Optional warning and punishment events |
| `GEOFENCE_ERROR_LOG_CHANNEL_ID` | empty | Optional worker and RCON errors |
| `ALLOWED_USER_IDS` | empty | Optional comma-separated button-user allowlist |
| `ALLOWED_ROLE_IDS` | empty | Optional comma-separated button-role allowlist |
| `STATUS_REFRESH_SECONDS` | `20` | Panel refresh interval, 10–300 seconds |
| `STATUS_REFRESH_TIMEOUT_SECONDS` | `8` | Immediate RCON refresh timeout, 2–30 seconds |
| `WORKER_SHUTDOWN_SECONDS` | `15` | Graceful profile-switch timeout, 1–30 seconds |
| `HLL_SERVER_LABEL[_N]` | `Server N` | Human-readable Discord panel name |
| `HLL_HOST`, `HLL_PORT`, `HLL_PASSWORD` | required | First server's RCON connection |
| `HLL_HOST_N`, `HLL_PORT_N`, `HLL_PASSWORD_N` | unset | Additional contiguous server tuples (`_2`, `_3`, ...)|
| `GEOFENCE_UID`, `GEOFENCE_GID` | `1000` | Owner IDs for writable `bot-data` |

The three player-limit values accept `0` through `100`. They are inclusive:
with `GEOFENCE_MIDCAP_MAX_PLAYERS=60`, players are punished at 60 connected
players and the midcap fences stop matching at 61.

Select exactly one value through `GEOFENCE_PROFILE`:

| Profile | Default active player range | Limit variable |
|---|---:|---|
| `midcap` | 0–60 | `GEOFENCE_MIDCAP_MAX_PLAYERS` |
| `midcap-advanced` | 0–99 | `GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS` |
| `lastcap` | 0–90 | `GEOFENCE_LASTCAP_MAX_PLAYERS` |

The Discord supervisor regenerates a server's credential-free policy
automatically whenever a profile button is pressed. Restart the container
after changing `.env` limits or other runtime settings. The manual generator
is needed only when running the Go worker without Discord.

## Language selection

Set `GEOFENCE_LANGUAGE` in `.env`:

- `en` selects English warning and punishment messages;
- `de` selects German warning and punishment messages.

English is the default. Any other value is rejected. Set
`GEOFENCE_WARNING_MESSAGE` and `GEOFENCE_PUNISH_MESSAGE` for custom single-line
texts. When the warning override is unset, its language-specific default
automatically includes the configured punishment delay. The same language
also controls Discord panels, button responses, and log embeds.

## Multiple servers

One policy can be applied to multiple servers without copying credentials into
YAML. Add contiguous environment tuples:

```dotenv
HLL_HOST_2=192.0.2.11
HLL_PORT_2=7779
HLL_PASSWORD_2=replace-with-second-rcon-password
HLL_SERVER_LABEL_2="HLL Vietnam Server 2"
GEOFENCE_CHANNEL_S2_ID=123456789012345682

HLL_HOST_3=192.0.2.12
HLL_PORT_3=7779
HLL_PASSWORD_3=replace-with-third-rcon-password
HLL_SERVER_LABEL_3="HLL Vietnam Server 3"
GEOFENCE_CHANNEL_S3_ID=123456789012345683
```

The supervisor creates one isolated worker per tuple. Every worker receives
only its own unsuffixed RCON credentials, so credentials from another server
cannot leak into its process. Missing fields/channels, invalid ports, duplicate
Discord channels, and numbering gaps are rejected before Discord login or any
RCON connection.

## Docker Compose

Set `GEOFENCE_UID` and `GEOFENCE_GID` in `.env` to the local owner of
`bot-data`, then run:

```bash
docker compose up -d --build
docker compose logs -f geofences
```

Compose passes `.env` directly to the container and mounts only the persistent
`bot-data` directory. The container runs without root privileges, with all
Linux capabilities dropped, a read-only root filesystem, and no Docker socket.
It restarts automatically unless explicitly stopped.

Stop it with:

```bash
docker compose down
```

## Operational security and privacy

- Never commit `.env`; keep it mode `0600` and rotate a Discord token or RCON
  password immediately if it is exposed.
- Restrict the game server's RCON port at the firewall to the geofence host or
  another explicitly trusted management network. Do not expose RCON broadly to
  the internet.
- Optional Discord event logs can contain player names and stable player IDs.
  Operators are responsible for access controls, retention, and applicable
  privacy requirements. Leave the log channel variables empty when forwarding
  this data is not needed.
- Treat control-channel access as administrative access: profile changes can
  alter enforcement immediately and the worker can punish connected players.
- Test new geometry and player limits on a non-production server first.

## Manual operation without Discord

The standalone Go worker remains available for operators who deliberately do
not want Discord. Generate one credential-free policy:

```bash
./scripts/generate-config.sh
go run ./cmd/configcheck config.yml
go run ./cmd
```

This manual mode applies one selected policy to all server tuples and loads
`.env` plus `config.yml` from the working directory.

## Manual policy maintenance

The generator uses the following atomic command internally:

```bash
go run ./cmd/applymap --map vietnam --variant midcap --max-players 60 config.yml
```

It removes previous fences and installs the canonical Vietnam rules. The
`--max-players` value is inclusive and accepts `0` through `100`. Individual
maps can be reapplied idempotently with `--map van-tuong`, `quang-ngai`,
`hue-outskirts`, `dak-to-airfield`, `cam-ranh-port`, or `thanh-hoa`. The final
policy must still pass the Vietnam-only validator.

## Verification

Run the same checks used by CI:

```bash
gofmt -w cmd/*.go cmd/applymap/*.go cmd/configcheck/*.go data/*.go maprules/*.go sync/*.go worker/*.go
go mod verify
go mod tidy -diff
go list -mod=readonly ./...
go test ./... -count=1
go vet ./...
npm ci --ignore-scripts --no-audit --no-fund
npm test
npm run check
npm audit --omit=dev --audit-level=high
bash -n scripts/generate-config.sh tests/generate-config.test.sh
./tests/generate-config.test.sh
go build ./...
docker build -t hllv-geofences:local .
```

## HLL: Vietnam RCON dependency

The application uses a pinned work-in-progress revision of `go-hll-rcon` for
HLL: Vietnam. An audited compatibility copy is included under
`third_party/go-hll-rcon` and selected through the `replace` directive in
`go.mod` so a fresh clone builds without another local repository. The copy
contains documented local fixes and preserves the upstream MIT license.

See `CONTRIBUTING.md` for map-change requirements and `SECURITY.md` for private
vulnerability reporting guidance.

See `RELEASING.md` for the final owner-specific GitHub publication gate and
release checklist.

## License

The project is licensed under MIT. Third-party components retain their own
compatible licenses. See `LICENSE`, `THIRD_PARTY_NOTICES.md`, and the license
included with the modified compatibility dependency.
