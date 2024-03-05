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

	// source directory
	dir := "."
	if len(os.Args) > 1 {
		dir, err = filepath.Abs(os.Args[1])
		if err != nil {
			log.Fatalf("fail to resolve absolute path of %s: %v", os.Args[1], err)
		}
	}
	src := client.Host().Directory(dir, dagger.HostDirectoryOpts{Exclude: []string{".git"}})

	// start postgres container
	postgres := client.Container().
		From("postgres:16").
		WithEnvVariable("POSTGRES_PASSWORD", "testing").
		WithEnvVariable("POSTGRES_USER", "testing").
		WithEnvVariable("POSTGRES_DB", "testing").
		WithMountedFile("/docker-entrypoint-initdb.d/schema.sql", src.File("schema.sql"))
	postgresService, err := postgres.AsService().Start(ctx)
	if err != nil {
		log.Fatalf("fail to start postgres service %v", err)
	}

	// build docker image for running test and run the test
	image := src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Target: "builder",
	})
	test := image.
		WithMountedCache("/go/pkg/mod", client.CacheVolume("golang-mod")).
		WithMountedCache("/root/.cache/go-build", client.CacheVolume("golang-build")).
		WithServiceBinding("postgres", postgresService).
		WithEnvVariable("POSTGRES_URL", "postgres://testing:testing@postgres:5432/testing?sslmode=disable").
		WithMountedDirectory(".", src).
		WithExec(
			[]string{"go", "test", "-coverprofile", "coverage.out", "./..."},
			dagger.ContainerWithExecOpts{
				SkipEntrypoint: true,
			},
		)
	_, err = test.File("coverage.out").Export(ctx, filepath.Join(dir, "coverage.out"))
	if err != nil {
		log.Fatalf("fail to export coverage: %v", err)
	}
}
