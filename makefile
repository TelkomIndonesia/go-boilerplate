.PHONY: build test start stop

build: 
	go build ./...

test: 
	cd .dagger && go run . "$$(cd ..; pwd)"

start: 
	docker compose up --build -d --force-recreate

stop:
	docker compose down