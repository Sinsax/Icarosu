.PHONY: build test vet fmt docker-up

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.go/*' -not -path './MaiBot/*')

docker-up:
	docker compose up --build