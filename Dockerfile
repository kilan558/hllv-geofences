FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY third_party/go-hll-rcon ./third_party/go-hll-rcon
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -trimpath -ldflags="-s -w" -o /out/hllv-geofences ./cmd/cmd.go
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -trimpath -ldflags="-s -w" -o /out/applymap ./cmd/applymap

FROM node:25-alpine@sha256:bdf2cca6fe3dabd014ea60163eca3f0f7015fbd5c7ee1b0e9ccb4ced6eb02ef4 AS node-dependencies
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund

FROM node:25-alpine@sha256:bdf2cca6fe3dabd014ea60163eca3f0f7015fbd5c7ee1b0e9ccb4ced6eb02ef4
WORKDIR /app
LABEL org.opencontainers.image.title="HLL: Vietnam Geofences"
LABEL org.opencontainers.image.description="Standalone Discord-controlled Vietnam-only Warfare geofences"
LABEL org.opencontainers.image.version="1.0.0"
LABEL org.opencontainers.image.licenses="MIT"
ENV COMMUNITY_NAME="HLL: Vietnam"
ENV BOT_DATA_DIR="/app/data"
COPY --from=node-dependencies --chown=node:node /src/node_modules ./node_modules
COPY --chown=node:node bot ./bot
COPY --chown=node:node package.json ./package.json
COPY --chown=node:node LICENSE THIRD_PARTY_NOTICES.md ./licenses/
COPY --chown=node:node third_party/licenses ./licenses/third-party
COPY --chown=node:node third_party/go-hll-rcon/LICENSE ./licenses/go-hll-rcon-LICENSE
COPY --from=node-dependencies --chown=node:node /src/node_modules/discord.js/LICENSE ./licenses/Apache-2.0.txt
COPY --from=build /out/hllv-geofences /usr/local/bin/hllv-geofences
COPY --from=build /out/applymap /usr/local/bin/applymap
RUN mkdir -p /app/data && chown node:node /app/data
USER 1000:1000
ENTRYPOINT ["node", "bot/main.mjs"]
