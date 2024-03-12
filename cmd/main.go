package main

import (
	"context"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
)

func main() {
	ctx := context.Background()

	c, err := cmd.NewServer(
		cmd.ServerWithCanceler(util.CancelOnExitSignal),
		cmd.ServerWithOtel(ctx),
	)
	if err != nil {
		logger.Global().Fatal("fail to instantiate server", logger.Any("error", err))
	}

	logger.Global().Info("server starting", logger.Any("server", c))
	if err = c.Run(ctx); err != nil {
		logger.Global().Fatal("error when running server", logger.Any("error", err))
	}
}
