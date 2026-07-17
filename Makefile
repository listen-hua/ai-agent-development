.PHONY: dev test build compose-up compose-down

dev:
	@echo "Run backend: cd backend && go run ./cmd/server"
	@echo "Run frontend: cd frontend && npm run dev"

test:
	cd backend && go test ./...
	cd frontend && npm run test:unit -- --run

build:
	cd backend && go build ./cmd/server && go build ./cmd/worker
	cd frontend && npm run build

compose-up:
	docker compose up --build

compose-down:
	docker compose down

