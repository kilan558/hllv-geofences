# Changelog

All notable changes to this project are documented in this file. Releases use
semantic versioning.

## 1.0.0 - 2026-08-22

### Added

- Standalone Vietnam-only Warfare geofence enforcement for all six bundled
  HLL: Vietnam maps.
- Independent Discord control panels for every configured HLL server.
- English and German runtime messages selected through `.env`.
- Configurable profiles, inclusive player limits, warning and punishment text,
  punishment delay, grid geometry, and player whitelist.
- Optional Discord event and error log channels.
- Credential-free generated policies with one environment credential tuple per
  HLL server.
- Persistent per-server profile state and live RCON status refresh.
- Rootless, read-only Docker deployment without a Docker socket, CRCON, Master
  Cache, or seeding mode.
- MIT project licensing and retained third-party license notices.

### Security

- Discord credentials and unrelated server tuples are removed from worker
  process environments.
- Discord mentions are disabled for relayed game data.
- The container drops every Linux capability and enables `no-new-privileges`.
