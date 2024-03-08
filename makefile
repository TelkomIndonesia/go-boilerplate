.PHONY: build test start stop

certs:
	docker run -it --rm  -v "$$PWD:$$PWD" -w "$$PWD/.local" --entrypoint "" jitesoft/step-cli \
		step certificate create ca ca.crt ca.key --profile root-ca --no-password --insecure -f;
	docker run -it --rm  -v "$$PWD:$$PWD" -w "$$PWD/.local" --entrypoint "" jitesoft/step-cli \
		step certificate create localhost localhost.crt localhost.key --profile leaf --ca ca.crt --ca-key ca.key --no-password --insecure -f;
	go run ./tools/gentinkey .local/tink-aead.json .local/tink-mac.json

build: 
	go build ./...

test: 
	cd .dagger && go run . "$$(cd ..; pwd)"

start: certs
	docker compose up --build -d --force-recreate

stop:
	docker compose down