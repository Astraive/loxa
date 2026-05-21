FROM golang:1.25-bookworm
WORKDIR /workspace
COPY . .
RUN go mod download
CMD ["go", "test", "./..."]
