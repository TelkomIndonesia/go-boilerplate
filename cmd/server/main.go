package main

import (
	"context"
	"log"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/otel"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

func main() {
	ctx := util.CancelOnExitSignal(context.Background())

	defer otel.FromEnv(ctx)()

	c, err := cmd.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	if err = c.Run(ctx); err != nil {
		log.Println(err)
	}
}
