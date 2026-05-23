.PHONY: proto lint clean check-proto

# Directories
PROTO_DIR := proto
GEN_DIR := gen/go
PROTO_FILES := $(wildcard $(PROTO_DIR)/loxa/v1/*.proto)
GEN_FILES := $(patsubst $(PROTO_DIR)/loxa/v1/%.proto,$(GEN_DIR)/loxa/v1/%.pb.go,$(PROTO_FILES)) \
             $(patsubst $(PROTO_DIR)/loxa/v1/%.proto,$(GEN_DIR)/loxa/v1/%_grpc.pb.go,$(PROTO_FILES))

# Generate Go protobuf code from proto/loxa/v1/*.proto into gen/go/loxa/v1/
# Uses protoc directly (buf is optional, see proto/buf.gen.yaml for buf config)
proto:
	protoc -I $(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		proto/loxa/v1/event.proto \
		proto/loxa/v1/ingest.proto \
		proto/loxa/v1/collector.proto \
		proto/loxa/v1/cortex.proto

# Lint protobuf definitions (requires buf)
lint:
	cd $(PROTO_DIR) && buf lint

# Clean generated Go protobuf code
clean:
	rm -rf $(GEN_DIR)/loxa/v1/*.pb.go $(GEN_DIR)/loxa/v1/*_grpc.pb.go

# Check that generated proto files are up to date with source
# Saves current files, regenerates, then compares byte-by-byte.
# This works regardless of gitignore on gen/go/.
check-proto:
	@echo "Checking generated proto files are up to date..."
	@mkdir -p /tmp/proto-backup
	@cp $(GEN_DIR)/loxa/v1/*.pb.go $(GEN_DIR)/loxa/v1/*_grpc.pb.go /tmp/proto-backup/ 2>/dev/null || true
	@$(MAKE) proto
	@for f in /tmp/proto-backup/*.pb.go; do \
		basename=$$(basename $$f); \
		if ! cmp -s "$$f" "$(GEN_DIR)/loxa/v1/$$basename"; then \
			echo "ERROR: $$basename is out of date. Run 'make proto' and commit the changes."; \
			rm -rf /tmp/proto-backup; \
			exit 1; \
		fi \
	done
	@rm -rf /tmp/proto-backup
	@echo "All generated proto files are up to date."
