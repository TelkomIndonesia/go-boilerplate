package main

import (
	"context"

	"github.com/telkomindonesia/go-boilerplate/internal/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

func main() {
	ctx := context.Background()
	c, err := cmd.New()
	if err != nil {
		log.Global().Fatal(ctx, "failed to instantiate server", log.Error("error", err))
	}

	if err = c.Run(ctx); err != nil {
		log.Global().Fatal(ctx, "error when running server", log.Error("error", err))
	}
}
