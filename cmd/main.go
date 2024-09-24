package main

import (
	"context"

	"github.com/telkomindonesia/go-boilerplate/internal/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func main() {
	c, err := cmd.New()
	if err != nil {
		log.Global().Fatal("failed to instantiate server", log.Error("error", err))
	}

	if err = c.Run(context.Background()); err != nil {
		log.Global().Fatal("error when running server", log.Error("error", err))
	}
}
