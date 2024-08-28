.PHONY:

keys:
	go run ./tools/gentinkey .local/tink-aead.json .local/tink-mac.json .local/tink-jwt-mac.json
	docker compose -f docker-compose.keys.yml up
	docker compose -f docker-compose.keys.yml down

generate:
	go generate ./...
	go mod tidy

build: 
	go build ./...

test: 
	cd .dagger && go run . ..

ci:
	IMAGE=$$(DOCKER_BUILDKIT=1 docker build -f .dagger/Dockerfile --label test -q .dagger)
	docker run --rm -i \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$$(pwd):$$(pwd)" -w "$$(pwd)" \
		$${IMAGE}
	docker rmi -f $$(docker images -q --filter=label=test) || true

start:
	docker compose up --build profile

debug:
	PROFILE_DOCKERFILE_TARGET=debugger docker compose up --build profile

stop:
	docker compose down

purge:
	docker compose -f docker-compose.yml down --volumes

sqlc-vet: 
	 SQLCDEBUG=dumpexplain=1 go run github.com/sqlc-dev/sqlc/cmd/sqlc vet -f ./pkg/postgres/sqlc.yaml