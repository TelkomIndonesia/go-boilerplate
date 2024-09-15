package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

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
		log.Fatalf("failed to resolve absolute path of %s: %v", dir, err)
	}
	src := client.Host().Directory(dir,
		dagger.HostDirectoryOpts{Exclude: []string{".git", ".dagger"}},
	)

	// start postgres container
	postgres := client.Container().
		From("postgres:16").
		WithEnvVariable("POSTGRES_PASSWORD", "testing").
		WithEnvVariable("POSTGRES_USER", "testing").
		WithEnvVariable("POSTGRES_DB", "testing").
		WithMountedFile("/docker-entrypoint-initdb.d/schema.sql", src.File("pkg/postgres/schema.sql")).
		WithMountedFile("/docker-entrypoint-initdb.d/outboxce.sql", src.File("pkg/util/outboxce/postgres/schema.sql")) // outboxce
	postgresService, err := postgres.AsService().Start(ctx)
	if err != nil {
		log.Fatalf("failed to start postgres service %v", err)
	}

	// start kafka container
	kafka := client.Container().
		From("bitnami/kafka:latest").
		WithEnvVariable("KAFKA_CFG_NODE_ID", "0").
		WithEnvVariable("KAFKA_CFG_PROCESS_ROLES", "controller,broker").
		WithEnvVariable("KAFKA_CFG_LISTENERS", "INTERNAL://:9092,EXTERNAL://:19092,CONTROLLER://:9093").
		WithEnvVariable("KAFKA_CFG_ADVERTISED_LISTENERS", "INTERNAL://kafka:9092,EXTERNAL://kafka:19092").
		WithEnvVariable("KAFKA_CFG_INTER_BROKER_LISTENER_NAME", "INTERNAL").
		WithEnvVariable("KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP", "CONTROLLER:PLAINTEXT,INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT").
		WithEnvVariable("KAFKA_CFG_CONTROLLER_QUORUM_VOTERS", "0@localhost:9093").
		WithEnvVariable("KAFKA_CFG_CONTROLLER_LISTENER_NAMES", "CONTROLLER")
	kafkaService, err := kafka.AsService().Start(ctx)
	if err != nil {
		log.Fatalf("failed to start redpanda service %v", err)
	}

	// wait postgres and kafka
	waiter := client.Container().From("alpine").
		WithServiceBinding("postgres", postgresService).
		WithServiceBinding("kafka", kafkaService).
		WithEntrypoint([]string{"sh", "-c"}).
		WithEnvVariable("_NOW_", time.Now().String()).
		WithExec([]string{`
			until nc postgres 5432; do echo "wait postgres"; sleep 1; done
			until nc kafka 9092; do echo "wait kafka"; sleep 1; done
		`})

	// build docker image for running test and run the test
	image := src.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Target: "base",
	})
	test := image.
		WithMountedCache("/go/pkg/mod", client.CacheVolume("golang-mod")).
		WithMountedCache("/root/.cache/go-build", client.CacheVolume("golang-build")).
		WithServiceBinding("postgres", postgresService).
		WithEnvVariable("TEST_POSTGRES_URL", "postgres://testing:testing@postgres:5432/testing?sslmode=disable").
		WithServiceBinding("kafka", kafkaService).
		WithEnvVariable("TEST_KAFKA_BROKERS", "kafka:9092").
		WithMountedDirectory("/tmp/waiter", waiter.Directory("/")).
		WithWorkdir(dir).
		WithMountedDirectory(".", src).
		WithExec(
			[]string{"go", "test", "-coverprofile", "coverage.out", "./..."},
			dagger.ContainerWithExecOpts{
				SkipEntrypoint: true,
			},
		)
	_, err = test.File("coverage.out").Export(ctx, filepath.Join(dir, "coverage.out"))
	if err != nil {
		log.Fatalf("failed to export coverage: %v", err)
	}
}
