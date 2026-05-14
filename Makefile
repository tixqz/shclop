IMAGE ?= shclop:latest
BINARY ?= bin/shclop

.PHONY: test web-install web-build build docker-build helm-template bootstrap-check verify clean

test:
	go test ./...

web-install:
	cd web && npm install

web-build: web-install
	cd web && npm run build

build: web-build
	mkdir -p bin
	go build -o $(BINARY) ./cmd/shclop

docker-build:
	docker build -t $(IMAGE) .

helm-template:
	helm template shclop charts/shclop

bootstrap-check:
	scripts/bootstrap.sh check --dry-run

verify: test web-build helm-template bootstrap-check

clean:
	rm -rf bin web/dist web/node_modules
