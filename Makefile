.PHONY: build test vet fmt run run-sdk docker-up

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.go/*' -not -path './MaiBot/*')

run:
	go run ./cmd/bot -config configs/config.yaml

run-sdk:
	go run ./cmd/icarosu -config configs/config.yaml -plugins-dir configs/plugins

docker-up:
	docker compose up --build