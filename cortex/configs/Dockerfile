FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install protoc and Go protobuf plugins for proto generation
RUN apk add --no-cache protobuf-dev && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5

# Copy module files and download dependencies
COPY cortex/go.mod cortex/go.sum ./cortex/
RUN cd cortex && go mod download

# Copy the entire repo (proto/ and gen/ are needed for proto generation)
COPY . .

# Generate protobuf code from proto/loxa/v1/ into gen/go/loxa/v1/
RUN make proto

# Build the cortex binary
RUN cd cortex && CGO_ENABLED=1 go build -o /build/cortex ./cmd/server

FROM alpine:3.20

RUN apk --no-cache add ca-certificates libc6-compat

WORKDIR /app

COPY --from=builder /build/cortex .
COPY --from=builder /build/cortex/configs/*.yaml ./configs/ 2>/dev/null || true

EXPOSE 8080 9090

ENV CORTEX_SERVER_HOST=0.0.0.0
ENV CORTEX_SERVER_PORT=8080
ENV CORTEX_STORAGE_BACKEND=postgres
ENV CORTEX_LOG_LEVEL=info

ENTRYPOINT ["/app/cortex"]