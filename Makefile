.PHONY: up up-all down down-volumes build test migrate proto install-tools lint

# Variables
PROTO_DIR     = ./proto
GEN_DIR       = ./gen
GOPATH_BIN    = $(shell go env GOPATH)/bin
GATEWAY_MOD   = ./third_party/googleapis

# Infrastructure only (default — does not start app services)
up:
	docker compose up -d

# Infrastructure + all app services
up-all:
	docker compose --profile app up -d --build

# Stop all services (infra + app)
down:
	docker compose --profile app down

# Stop all services and remove data volumes (clean reset)
down-volumes:
	docker compose --profile app down -v

# Build app service Docker images
build:
	docker compose --profile app build

# Run all Go tests with race detector
test:
	go test ./... -v -race -count=1

# Services run migrations on startup via golang-migrate embedded in each binary.
# Use 'make up-all' to start services (they migrate automatically on boot).
migrate:
	@echo "Services run migrations on startup. Use 'make up-all' to start services with migrations."

# Generate protobuf Go code for all .proto files
proto:
	@mkdir -p $(GEN_DIR)
	@PATH="$(GOPATH_BIN):$(PATH)" && find $(PROTO_DIR) -name "*.proto" | while read f; do \
		protoc \
			--proto_path=$(PROTO_DIR) \
			--proto_path=$(GATEWAY_MOD) \
			--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
			--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
			--grpc-gateway_out=$(GEN_DIR) --grpc-gateway_opt=paths=source_relative \
			"$$f"; \
	done
	@echo "Proto generation complete"

# Install protoc and all required Go generator plugins
install-tools:
	@which protoc > /dev/null || (echo "Installing protoc..." && brew install protobuf)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	@echo "Tools installed successfully"

# Run static analysis
lint:
	go vet ./...

# Regenerate Swagger docs for API Gateway
swagger:
	cd services/api-gateway && go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go -o docs/ --parseDepth 1 --parseDependency

## k6 load testing
test-smoke:
	docker run --rm -i --network host grafana/k6 run - < tests/k6/smoke.js

test-load:
	docker run --rm -i --network host grafana/k6 run - < tests/k6/load.js

test-stress:
	docker run --rm -i --network host grafana/k6 run - < tests/k6/stress.js

# DATA-02: one-time cleanup of load-test orders (idempotent, safe to re-run).
clean-loadtest:
	docker compose exec -T postgres psql -U tracker -d tracker -f /dev/stdin < scripts/cleanup-loadtest-orders.sql
