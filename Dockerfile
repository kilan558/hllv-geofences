FROM golang:1.27.0-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY third_party/go-hll-rcon ./third_party/go-hll-rcon
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -trimpath -ldflags="-s -w" -o /out/hllv-geofences ./cmd/cmd.go
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -trimpath -ldflags="-s -w" -o /out/applymap ./cmd/applymap

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS node-dependencies
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32
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
