# syntax=docker/dockerfile:1.7
#
# Two-stage build:
#   1. golang:alpine compiles every binary into /out
#   2. alpine:latest runs them (small final image, ~25 MB)
#
# The container is meant for the validate / smoke / example binaries.
# Pass LLM credentials via -e at runtime; nothing is baked in.

FROM golang:1.23-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src

# Cache dependency download as a separate layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /out && \
    go build -trimpath -ldflags="-s -w" -o /out/validate ./cmd/validate && \
    go build -trimpath -ldflags="-s -w" -o /out/smoke    ./cmd/smoke && \
    for ex in engineer paper-write battle-auto-sop battle-custom-sop werewolf undercover-auto-sop undercover-custom-sop; do \
      go build -trimpath -ldflags="-s -w" -o /out/$ex ./examples/$ex; \
    done

FROM alpine:3.20
RUN apk add --no-cache ca-certificates bash python3 && \
    addgroup -S app && adduser -S -G app -u 1000 app
WORKDIR /home/app
COPY --from=build /out/ /usr/local/bin/
USER app
ENV PATH=/usr/local/bin:$PATH

# Default to the no-credentials validation — overrideable.
CMD ["validate"]
