package env

import (
	"fmt"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type Options struct {
	Prefix string
	DotEnv bool
}

func Load(v interface{}, opt Options) (err error) {
	if opt.DotEnv {
		godotenv.Load()
	}

	envopt := env.Options{Prefix: opt.Prefix}
	if err = env.ParseWithOptions(v, envopt); err != nil {
		return fmt.Errorf("failed to load env: %w", err)
	}
	return nil
}
