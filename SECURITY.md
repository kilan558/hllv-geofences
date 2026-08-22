# Security Policy

## Reporting a vulnerability

Do not disclose vulnerabilities or credentials in a public issue. Use GitHub's
private vulnerability reporting feature when it is enabled for the repository.
If it is unavailable, open a public issue asking the maintainer for a private
contact method without including technical details, credentials, logs, or
screenshots. Rotate exposed Discord bot tokens and RCON passwords immediately;
deleting them from a later commit does not revoke them.

Include the affected revision, impact, reproduction steps, and any suggested
mitigation. Remove RCON passwords, player identifiers, server addresses, and
Discord identifiers from logs and screenshots.

## Operational scope

The service can warn and punish players through RCON. Test new map geometry on
a non-production server, validate every generated configuration with
`cmd/configcheck`, and keep generated configuration files readable only by the
runtime account. Grant the Discord bot only View Channel, Send Messages, Embed
Links, and Read Message History in its dedicated channels. The standalone
supervisor does not require Administrator, Message Content, Guild Members, a
Docker socket, CRCON, or Master Cache access.

Restrict the RCON port to the geofence host or another trusted management
network. Optional Discord event logs may contain player names and stable player
identifiers; operators must apply appropriate channel access, retention, and
privacy controls.
