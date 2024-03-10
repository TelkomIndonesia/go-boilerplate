package main

import (
	"context"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/otel"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

func main() {
	ctx := util.CancelOnExitSignal(context.Background())

	defer otel.FromEnv(ctx)()

	c, err := cmd.NewServer()
	if err != nil {
		logger.Global().Fatal("fail to instantiate server", logger.Any("error", err.Error()))
	}
	if err = c.Run(ctx); err != nil {
		logger.Global().Fatal("error when running server", logger.Any("error", err.Error()))
	}
}
