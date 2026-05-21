FROM golang:1.25-bookworm AS builder

WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends build-essential pkg-config ca-certificates && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /out/loxa-collector ./cmd/loxa-collector

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/loxa-collector /usr/local/bin/loxa-collector
COPY deploy/docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY loxa-collector.defaults.yaml /app/config.yaml

RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 9090

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["loxa-collector", "run", "-c", "/app/config.yaml"]
