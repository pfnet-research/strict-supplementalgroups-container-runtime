package config

import (
	"github.com/mcuadros/go-defaults"
	"github.com/rs/zerolog"
)

func getDefaultConfig() *Config {
	cfg := &Config{Logging: LogConfig{LogLevel: zerolog.InfoLevel}}
	defaults.SetDefaults(cfg)
	return cfg
}
