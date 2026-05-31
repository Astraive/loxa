FROM golang:1.25-bookworm AS builder

WORKDIR /build

# Install protoc and Go protobuf plugins for proto generation
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential pkg-config ca-certificates protobuf-compiler libprotobuf-dev \
    && rm -rf /var/lib/apt/lists/*
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5

# Copy module files and download dependencies
COPY gen/go/go.mod gen/go/go.sum ./gen/go/
COPY spec/go.mod ./spec/go.mod
COPY collector/go.mod collector/go.sum ./collector/
RUN cd collector && go mod download

# Copy the entire repo (proto/ and gen/ are needed for proto generation)
COPY . .

# Generate protobuf code from proto/loxa/core/ into gen/go/loxa/core/
RUN make proto

# Build the collector binary
RUN cd collector && CGO_ENABLED=1 GOOS=linux go build -o /out/loxa-collector ./cmd/loxa-collector

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata wget && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/loxa-collector /usr/local/bin/loxa-collector
COPY collector/deploy/docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY collector/loxa-collector.defaults.yaml /app/config.yaml

RUN chmod +x /usr/local/bin/docker-entrypoint.sh && \
    useradd -r -s /bin/false -u 10001 loxa && \
    chown -R loxa:loxa /app

USER loxa

EXPOSE 9308 9309 9310 9311

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --spider -q http://localhost:9308/health || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["loxa-collector", "run", "-c", "/app/config.yaml"]
