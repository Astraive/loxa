FROM golang:1.25 AS builder

WORKDIR /build

# Install protoc and Go protobuf plugins for proto generation
RUN apt-get update && apt-get install -y --no-install-recommends \
        make \
        protobuf-compiler \
        libprotobuf-dev \
        g++ \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5

# Copy module files and local replacement module manifests, then download deps.
COPY cortex/go.mod cortex/go.sum ./cortex/
COPY gen/go/go.mod ./gen/go/go.mod
COPY spec/go.mod ./spec/go.mod
RUN cd cortex && go mod download

# Copy the entire repo (proto/ and gen/ are needed for proto generation)
COPY . .

# Generate protobuf code from proto/loxa/core/ into gen/go/loxa/core/
RUN protoc -I proto -I /usr/include \
    --go_out=gen/go --go_opt=paths=source_relative \
    --go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
    proto/loxa/core/event.proto \
    proto/loxa/core/ingest.proto \
    proto/loxa/core/collector.proto \
    proto/loxa/core/cortex.proto

# Build the cortex binary
RUN cd cortex && CGO_ENABLED=1 go build -o /build/loxa-cortex ./cmd/server

FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/loxa-cortex ./cortex
COPY --from=builder /build/cortex/configs ./configs

EXPOSE 8080 9090

ENV CORTEX_SERVER_HOST=0.0.0.0
ENV CORTEX_SERVER_PORT=8080
ENV CORTEX_STORAGE_BACKEND=postgres
ENV CORTEX_LOG_LEVEL=info

RUN useradd -r -s /bin/false -u 10001 loxa && chown -R loxa:loxa /app
USER loxa

ENTRYPOINT ["/app/cortex"]
