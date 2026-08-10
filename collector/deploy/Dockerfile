FROM golang:1.25-bookworm
WORKDIR /workspace
COPY . .
RUN go mod download
RUN useradd -r -s /bin/false -u 10001 loza && chown -R loza:loza /workspace
USER loza

CMD ["go", "test", "./..."]
