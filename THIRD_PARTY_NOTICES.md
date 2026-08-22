# Third-party notices

This project is distributed under the MIT License in `LICENSE`. Third-party
components remain subject to their respective licenses; the project license
does not replace those terms.

## Go runtime dependencies

| Component | Version | License | Included notice |
|---|---|---|---|
| `github.com/floriansw/go-hll-rcon` | `057e2ffe848e` | MIT | `third_party/go-hll-rcon/LICENSE` |
| `github.com/joho/godotenv` | `v1.5.1` | MIT | `third_party/licenses/godotenv-LICENCE` |
| `gopkg.in/yaml.v3` | `v3.0.1` | MIT and Apache-2.0 | `third_party/licenses/yaml-v3-LICENSE` and `yaml-v3-NOTICE` |

`go-hll-rcon` is an audited local compatibility copy selected by the `replace`
directive in `go.mod`. It is based on commit `057e2ffe848e`, removes files that
are not needed at runtime, and contains local HLL: Vietnam compatibility
changes. Its original MIT copyright and license are preserved.

Additional modules in `go.mod` and `go.sum` are used for tests or tooling and
are fetched by Go rather than redistributed in this repository. Their license
files remain in their respective modules.

## Node.js runtime dependencies

The production dependency tree locked by `package-lock.json` contains these
license families:

- Apache-2.0: `discord.js`, `@discordjs/builders`, `@discordjs/collection`,
  `@discordjs/formatters`, `@discordjs/rest`, `@discordjs/util`, and
  `@discordjs/ws`;
- 0BSD: `tslib`;
- MIT: `@sapphire/async-queue`, `@sapphire/shapeshift`,
  `@sapphire/snowflake`, `@types/node`, `@types/ws`,
  `@vladfrangu/async_event_emitter`, `discord-api-types`, `fast-deep-equal`,
  `lodash`, `lodash.snakecase`, `magic-bytes.js`, `ts-mixer`, `undici`,
  `undici-types`, and `ws`.

The packages and their license files are installed from the npm lockfile into
the container. The shared Sapphire MIT notice is retained separately at
`third_party/licenses/sapphire-MIT-LICENSE` because some Sapphire package
tarballs do not contain their own license file. The image also includes this
notice, the project license, the Go dependency notices, and a full Apache-2.0
license at `/app/licenses`.

Dependency versions and license metadata were audited on 2026-08-22 with
`go list -deps -json`, `go mod verify`, `npm ci`, `npm ls --omit=dev --all`, and
`npm query '*' --omit=dev`. Re-run the audit whenever either lockfile changes.
