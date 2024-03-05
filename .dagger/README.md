# CI/CD

## Run

From the root folder of this repository, run:

```bash
cd .dagger \
    && go build -o dagger . \
    && cd .. \
    && .dagger/dagger
```

### Using Docker

From the root folder of this repository, run:

```bash
docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v $PWD:$PWD -w $PWD \
    $(
        docker build -f .dagger/Dockerfile  -q .dagger 
    )
```
