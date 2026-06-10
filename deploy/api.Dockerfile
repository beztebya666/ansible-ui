# --- build the SPA ----------------------------------------------------
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# --- build the Go api with the SPA embedded ---------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY examples ./examples
# Replace the placeholder bundle with the real compiled SPA, then embed it.
RUN rm -rf internal/webui/dist
COPY --from=web /web/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api

# --- runtime ----------------------------------------------------------
FROM alpine:3.20
# Runs as root so it can manage the shared /data volume the runner also mounts.
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
