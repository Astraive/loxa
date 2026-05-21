PROTO_DIR := proto

.PHONY: lint proto

lint:
	buf lint --path $(PROTO_DIR)

proto:
	buf build --path $(PROTO_DIR)
