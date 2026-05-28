.PHONY: proto lint clean check-proto

# Directories
PROTO_DIR := proto
GEN_DIR := gen/go
PROTO_FILES := $(wildcard $(PROTO_DIR)/loxa/core/*.proto)
GEN_FILES := $(patsubst $(PROTO_DIR)/loxa/core/%.proto,$(GEN_DIR)/loxa/core/%.pb.go,$(PROTO_FILES)) \
             $(patsubst $(PROTO_DIR)/loxa/core/%.proto,$(GEN_DIR)/loxa/core/%_grpc.pb.go,$(PROTO_FILES))

# Generate Go protobuf code from proto/loxa/core/*.proto into gen/go/loxa/core/
# Uses protoc directly (buf is optional, see proto/buf.gen.yaml for buf config)
proto:
	@mkdir -p $(GEN_DIR)/loxa/core
	protoc -I $(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		proto/loxa/core/event.proto \
		proto/loxa/core/ingest.proto \
		proto/loxa/core/collector.proto \
		proto/loxa/core/cortex.proto

# Lint protobuf definitions (requires buf)
lint:
	cd $(PROTO_DIR) && buf lint

# Clean generated Go protobuf code
clean:
	rm -rf $(GEN_DIR)/loxa/core/*.pb.go $(GEN_DIR)/loxa/core/*_grpc.pb.go

# Check that generated proto files are up to date with source
# Saves current files, regenerates, then compares byte-by-byte.
# This works regardless of gitignore on gen/go/.
check-proto:
	@echo "Checking generated proto files are up to date..."
	@mkdir -p /tmp/proto-backup
	@cp $(GEN_DIR)/loxa/core/*.pb.go $(GEN_DIR)/loxa/core/*_grpc.pb.go /tmp/proto-backup/ 2>/dev/null || true
	@$(MAKE) proto
	@for f in /tmp/proto-backup/*.pb.go; do \
		basename=$$(basename $$f); \
		if ! cmp -s "$$f" "$(GEN_DIR)/loxa/core/$$basename"; then \
			echo "ERROR: $$basename is out of date. Run 'make proto' and commit the changes."; \
			rm -rf /tmp/proto-backup; \
			exit 1; \
		fi \
	done
	@rm -rf /tmp/proto-backup
	@echo "All generated proto files are up to date."
