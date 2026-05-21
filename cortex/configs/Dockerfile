FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o cortex ./cmd/server

FROM alpine:3.20

RUN apk --no-cache add ca-certificates libc6-compat

WORKDIR /app

COPY --from=builder /build/cortex .
COPY --from=builder /build/configs/*.yaml ./configs/ 2>/dev/null || true

EXPOSE 8080 9090

ENV CORTEX_SERVER_HOST=0.0.0.0
ENV CORTEX_SERVER_PORT=8080
ENV CORTEX_STORAGE_BACKEND=postgres
ENV CORTEX_LOG_LEVEL=info

ENTRYPOINT ["/app/cortex"]