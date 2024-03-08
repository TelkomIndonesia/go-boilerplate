.PHONY: build test start stop certs

step-cli := docker run -it --rm  -v "$$PWD:$$PWD" -w "$$PWD/.local" --entrypoint "step" jitesoft/step-cli
certs:
	$(step-cli) certificate create ca ca.crt ca.key --profile root-ca --no-password --insecure -f
	$(step-cli) certificate create localhost localhost.crt localhost.key --profile leaf --ca ca.crt --ca-key ca.key --no-password --insecure -f
	go run ./tools/gentinkey .local/tink-aead.json .local/tink-mac.json

build: 
	go build ./...

test: 
	cd .dagger && go run . ..

start: certs
	docker compose up --build --force-recreate

stop:
	docker compose down