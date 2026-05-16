IMAGE ?= shclop:latest
BINARY ?= bin/shclop
RUNTIME_BINARY ?= bin/shclop-runtime
RUNTIME_IMAGE_PREFIX ?= shclop-runtime

.PHONY: test web-install web-build build docker-build runtime-images helm-template bootstrap-check verify clean

test:
	go test ./...

web-install:
	cd web && npm install

web-build: web-install
	cd web && npm run build

build: web-build
	mkdir -p bin
	go build -o $(BINARY) ./cmd/shclop
	go build -o $(RUNTIME_BINARY) ./cmd/shclop-runtime

docker-build:
	docker build -t $(IMAGE) .

runtime-images:
	docker build -f runtime/nanoclaw/Dockerfile -t $(RUNTIME_IMAGE_PREFIX)-nanoclaw:latest .
	docker build -f runtime/nemoclaw/Dockerfile -t $(RUNTIME_IMAGE_PREFIX)-nemoclaw:latest .
	docker build -f runtime/openclaw/Dockerfile -t $(RUNTIME_IMAGE_PREFIX)-openclaw:latest .

helm-template:
	helm template shclop charts/shclop

bootstrap-check:
	scripts/bootstrap.sh check --dry-run

verify: test web-build helm-template bootstrap-check

clean:
	rm -rf bin web/dist web/node_modules
