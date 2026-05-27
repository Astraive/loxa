FROM golang:1.25-bookworm
WORKDIR /workspace
COPY . .
RUN go mod download
RUN useradd -r -s /bin/false -u 10001 loxa && chown -R loxa:loxa /workspace
USER loxa

CMD ["go", "test", "./..."]
