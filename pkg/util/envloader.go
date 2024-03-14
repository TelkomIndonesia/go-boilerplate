package util

import (
	"fmt"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type LoadEnvOptions struct {
	Prefix string
	DotEnv bool
}

func LoadFromEnv(v interface{}, opt LoadEnvOptions) (err error) {
	if opt.DotEnv {
		godotenv.Load()
	}

	envopt := env.Options{Prefix: opt.Prefix}
	if err = env.ParseWithOptions(v, envopt); err != nil {
		return fmt.Errorf("fail to load env: %w", err)
	}
	return nil
}
