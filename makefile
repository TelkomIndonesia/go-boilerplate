.PHONY: build test start stop certs

certs:
	go run ./tools/gentinkey .local/tink-aead.json .local/tink-mac.json
	docker compose up gencerts

build: 
	go build ./...

test: 
	cd .dagger && go run . ..

start:
	docker compose up --build

stop:
	docker compose down