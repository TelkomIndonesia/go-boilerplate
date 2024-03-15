.PHONY: build test start stop certs

keys:
	go run ./tools/gentinkey .local/tink-aead.json .local/tink-mac.json
	docker compose -f docker-compose.keys.yml up

build: 
	go build ./...

test: 
	cd .dagger && go run . ..

start:
	docker compose up --build profile

stop:
	docker compose down