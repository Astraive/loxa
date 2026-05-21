test:
	go test ./...

bench:
	go test -bench=. ./bench/...

lint:
	go vet ./...
	golangci-lint run --config deploy/.golangci.yml

PROTO_DIR := proto
PROTO_FILES := \
	$(PROTO_DIR)/loxa/v1/event.proto \
	$(PROTO_DIR)/loxa/v1/ingest.proto

generate:
	@command -v protoc >/dev/null 2>&1 || (echo "protoc is required"; exit 1)
	@command -v protoc-gen-go >/dev/null 2>&1 || (echo "protoc-gen-go is required"; exit 1)
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || (echo "protoc-gen-go-grpc is required"; exit 1)
	protoc -I $(PROTO_DIR) \
	  --go_out=. --go_opt=module=github.com/astraive/loxa-collector \
	  --go-grpc_out=. --go-grpc_opt=module=github.com/astraive/loxa-collector \
	  $(PROTO_FILES)
