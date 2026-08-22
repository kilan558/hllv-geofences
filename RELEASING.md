# Release process

## Repository identity

The canonical repository is `github.com/kilan558/hllv-geofences`. Keep the Go
module path, internal imports, package metadata, and `.github/CODEOWNERS`
aligned with this location if the repository is ever transferred or renamed.
Preserve `Copyright (c) 2025 Florian`, `Copyright (c) 2026 kilan558`, and all
third-party notices.

## Pre-release verification

Run:

```bash
gofmt -w cmd/*.go cmd/applymap/*.go cmd/configcheck/*.go data/*.go maprules/*.go sync/*.go worker/*.go
go mod verify
go mod tidy -diff
go list -mod=readonly ./...
go test ./... -count=1
go vet ./...
go build ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
npm ci --ignore-scripts --no-audit --no-fund
npm test
npm run check
npm audit --omit=dev --audit-level=high
bash -n scripts/generate-config.sh tests/generate-config.test.sh
./tests/generate-config.test.sh
GEOFENCE_ENV_FILE=.env.example docker compose --env-file .env.example config
docker build -t hllv-geofences:local .
```

Verify that `.env`, `config.yml`, `bot-data/`, logs, credentials, real server
addresses, Discord IDs, and player identifiers are absent before the first
commit.

## GitHub repository settings

After creating the repository:

1. Enable Dependabot alerts, secret scanning, push protection, code scanning,
   and private vulnerability reporting.
2. Create a ruleset for the default branch that requires the CI workflow,
   blocks force pushes and branch deletion, and requires review from code
   owners when more than one maintainer exists.
3. Restrict allowed GitHub Actions to required publishers and require actions
   to be pinned to a full commit SHA.
4. Subscribe maintainers to security-alert notifications.
5. Push the initial commit, wait for a green CI run, then create the `v1.0.0`
   release from that exact commit.
