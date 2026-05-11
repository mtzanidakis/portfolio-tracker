# syntax=docker/dockerfile:1

# --- frontend build stage ---
FROM node:25-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --silent
COPY web/ ./
RUN npm run build

# --- go build stage ---
FROM golang:1.26.3-alpine AS build
ARG VERSION=docker
WORKDIR /src

# Pre-fetch Go modules so subsequent source changes don't bust the cache.
COPY go.mod go.sum ./
RUN go mod download

# Copy source, then drop the freshly built frontend into the embed location.
COPY . .
RUN rm -rf ./internal/web/dist/*
COPY --from=web /web/dist/ ./internal/web/dist/

ENV CGO_ENABLED=0
ENV GOFLAGS=-buildvcs=false
RUN go build \
    -ldflags "-s -w -X github.com/mtzanidakis/portfolio-tracker/internal/version.Version=${VERSION}" \
    -o /out/ptd ./cmd/ptd
RUN go build \
    -ldflags "-s -w -X github.com/mtzanidakis/portfolio-tracker/internal/version.Version=${VERSION}" \
    -o /out/ptadmin ./cmd/ptadmin

# --- runtime ---
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /data \
    && addgroup -S -g 1000 pt \
    && adduser -S -u 1000 -G pt pt \
    && chown pt:pt /data

COPY --from=build /out/ptd /out/ptadmin /usr/local/bin/

USER pt:pt
VOLUME /data
EXPOSE 8082
ENTRYPOINT ["ptd"]
