FROM golang:1.25-bookworm AS builder
ARG LQL_VERSION=0.5.1
ARG TARGETARCH

WORKDIR /build

# Install protoc, Go protobuf plugins, and curl for the pinned LQL release binary.
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential pkg-config ca-certificates protobuf-compiler libprotobuf-dev curl \
    && rm -rf /var/lib/apt/lists/*
RUN case "$TARGETARCH" in \
      amd64) lql_arch=x64; lql_sha256=3e8eb6d42ea3cf864ca9af50dab37834ba8363e4a89253898e214746d047bd89 ;; \
      arm64) lql_arch=arm64; lql_sha256=afd287f33b7167047ef66d35cd2e18d4338b98e78da423848651f5cfe2ea69af ;; \
      *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac && \
    curl --fail --location --retry 3 \
      "https://github.com/Astraive/lql/releases/download/v${LQL_VERSION}/lql-linux-${lql_arch}" \
      --output /usr/local/bin/lql && \
    printf '%s  %s\n' "$lql_sha256" /usr/local/bin/lql | sha256sum --check --status && \
    chmod 0755 /usr/local/bin/lql

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5

# Copy module files and download dependencies
COPY gen/go/go.mod gen/go/go.sum ./gen/go/
COPY spec/go.mod ./spec/go.mod
COPY collector/go.mod collector/go.sum ./collector/
RUN cd collector && go mod download

COPY . .

# Generate protobuf code from proto/loza/core/ into gen/go/loza/core/
RUN make proto

# Build the collector binary
RUN cd collector && CGO_ENABLED=1 GOOS=linux go build -o /out/loza-collector ./cmd/loza-collector

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata wget && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/loza-collector /usr/local/bin/loza-collector
COPY --from=builder /usr/local/bin/lql /usr/local/bin/lql
COPY collector/deploy/docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY collector/loza-collector.defaults.yaml /app/config.yaml

RUN chmod +x /usr/local/bin/docker-entrypoint.sh && \
    useradd -r -s /bin/false -u 10001 loza && \
    chown -R loza:loza /app

USER loza

EXPOSE 9308 9309 9310 9311

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --spider -q http://localhost:9308/health || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["loza-collector", "run", "-c", "/app/config.yaml"]
