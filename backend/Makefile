.PHONY: build check test integration race migrate status generate
build:
	mkdir -p bin
	go build -trimpath -o bin/server ./cmd/server
	go build -trimpath -o bin/manage ./cmd/manage
	go build -trimpath -o bin/bootstrap ./cmd/bootstrap
check:
	go mod verify
	go vet ./...
test:
	go test ./...
integration:
	@test -n "$(INTEGRATION_DATABASE_URL)" || (echo 'Set INTEGRATION_DATABASE_URL to a disposable PostgreSQL instance'; exit 1)
	@test -n "$(PG_TEST_CONTAINER)" || (echo 'Set PG_TEST_CONTAINER for the backup restore drill'; exit 1)
	go test -count=1 ./...
race:
	go test -race -count=1 ./...
migrate:
	go run ./cmd/manage up
status:
	go run ./cmd/manage status
generate:
	python3 tools/generate_contract.py
