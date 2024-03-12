package main

import (
	"context"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
)

func main() {
	c, err := cmd.NewServer()
	if err != nil {
		logger.Global().Fatal("fail to instantiate server", logger.Any("error", err))
	}

	if err = c.Run(context.Background()); err != nil {
		logger.Global().Fatal("error when running server", logger.Any("error", err))
	}
}
