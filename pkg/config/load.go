package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog"
)

const (
	defaultConfigDir = "/etc/strict-supplementalgroups-container-runtime"
	configFileName   = "config.toml"
)

func LoadConfig() (*Config, error) {
	configPath := filepath.Join(defaultConfigDir, configFileName)
	config := getDefaultConfig()

	// load if config file exists
	if _, err := os.Stat(configPath); err == nil {
		configBytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("Failed to read config file %s: %v", configPath, err)
		}

		if err := toml.Unmarshal(configBytes, &config); err != nil {
			return nil, fmt.Errorf("Failed to parse config file %s: %v", configPath, err)
		}
	}

	if err := validateAndCompleteConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

func validateAndCompleteConfig(cfg *Config) error {
	var err error

	level, err := zerolog.ParseLevel(cfg.Logging.LogLevelStr)
	if err != nil {
		return fmt.Errorf("Failed to parse log-level %s: %v", cfg.Logging.LogLevelStr, err)
	}
	cfg.Logging.LogLevel = level

	format := cfg.Logging.LogFormat
	if !(format == "text" || format == "json") {
		return fmt.Errorf("log-format must be test or json")
	}

	return nil
}
