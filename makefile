.PHONY: cicd

cicd:
	cd .dagger \
    && go build -o dagger . \
    && cd .. \
    && .dagger/dagger