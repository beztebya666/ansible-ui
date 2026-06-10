# All-in-one image: API + runner + PostgreSQL in ONE container, for quick trials:
#   docker run -d -p 8080:8080 beztebya666/ansible-ui:v1.0.0   (or ghcr.io/beztebya666/ansible-ui)
# For production use docker-compose or the Helm chart (separate, scalable services).

# 1. Build the SPA, then the api binary (with the SPA embedded).
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api

# 2. Final image = the runner image (ansible + terraform/opentofu/pulumi/pwsh + the runner
#    binary) + PostgreSQL + the api binary + an entrypoint that starts all three.
FROM beztebya666/ansible-ui-runner:v1.0.0
USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends postgresql \
    && rm -rf /var/lib/apt/lists/*
COPY --from=gobuild /out/api /usr/local/bin/api
COPY deploy/aio-entrypoint.sh /usr/local/bin/aio-entrypoint.sh
RUN chmod +x /usr/local/bin/aio-entrypoint.sh
LABEL org.opencontainers.image.source="https://github.com/beztebya666/ansible-ui" \
      org.opencontainers.image.description="ansible-ui all-in-one: API + runner + PostgreSQL in one container (for quick trials)"
VOLUME ["/var/lib/postgresql/data", "/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/aio-entrypoint.sh"]
