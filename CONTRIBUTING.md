# Contributing

Contributions should preserve the Vietnam-only runtime boundary and must not
add unverified map geometry.

By submitting a contribution, you agree that it may be distributed under this
repository's MIT License. Do not remove existing copyright, attribution,
license, or third-party notice files.

## Development checks

Run all checks before opening a pull request:

```bash
gofmt -w cmd/*.go cmd/applymap/*.go cmd/configcheck/*.go data/*.go maprules/*.go sync/*.go worker/*.go
go mod verify
go mod tidy -diff
go list -mod=readonly ./...
go test ./... -count=1
go vet ./...
go build ./...
npm ci --ignore-scripts --no-audit --no-fund
npm test
npm run check
npm audit --omit=dev --audit-level=high
bash -n scripts/generate-config.sh tests/generate-config.test.sh
./tests/generate-config.test.sh
docker build -t hllv-geofences:local .
```

For behavior changes, add a regression test first and confirm that it fails
for the expected reason before implementing the change. Never commit `.env`,
generated files under `configs/`, `bot-data/`, RCON passwords, Discord tokens,
logs, or runtime state.

Map changes must include the exact HLL: Vietnam layer identifiers, calibration
evidence, team orientation, supported player threshold, and tests for both
allowed and blocked coordinates.
