package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"dagger.io/dagger"
)

func main() {
	ctx := context.Background()

	// initialize Dagger client
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		log.Fatalf("failed to instantiate dagger client :%v", err)
	}
	defer client.Close()

	// load source directory
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		log.Fatalf("fail to resolve absolute path of %s: %v", dir, err)
	}

	_, err = client.Container().
		From("jitesoft/step-cli").
		WithExec(
			[]string{"sh", "-c", `
				set -e
				step certificate create ca ca.crt ca.key --profile root-ca --no-password --insecure -f
				step certificate create localhost localhost.crt localhost.key --profile leaf --ca ca.crt --ca-key ca.key --no-password --insecure -f
			`},
			dagger.ContainerWithExecOpts{SkipEntrypoint: true}).
		Directory(".").
		Export(ctx, filepath.Join(dir, ".local"))
	if err != nil {
		log.Fatalf("fail to export directory: %v", err)
	}
}
