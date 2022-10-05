package main

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/config"
	ociruntime "github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/oci/runtime"
)

var (
	// injected in build time
	Version = ""

	logFile *lumberjack.Logger
)

func closeLogFile() {
	if logFile != nil {
		logFile.Close()
	}
}

func panicHandler() {
	if r := recover(); r != nil {
		zlog.Fatal().Interface("panic", r).Msg("Panic")
	}
}

func main() {
	defer closeLogFile()
	defer panicHandler()

	cfg, err := config.LoadConfig()
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load config")
	}

	// setup logger
	logFile = &lumberjack.Logger{
		Filename:   cfg.Logging.LogFile,
		MaxSize:    cfg.Logging.Rotation.MaxSizeInMB,
		MaxBackups: cfg.Logging.Rotation.MaxBackups,
		MaxAge:     cfg.Logging.Rotation.MaxAgeInDay,
		Compress:   cfg.Logging.Rotation.Compress,
	}
	var logOutput io.Writer = logFile
	if cfg.Logging.LogFormat == "text" {
		logOutput = zerolog.ConsoleWriter{Out: logFile, TimeFormat: time.RFC3339, NoColor: true}
	}
	zlog.Logger = zerolog.New(logOutput).With().Timestamp().Str("Execution", uuid.New().String()).Logger().Level(cfg.Logging.LogLevel)

	zlog.Info().Str("version", Version).Msg("Execution Start")
	zlog.Debug().Interface("config", cfg).Msg("Config loaded")

	// run the container runtime
	containerRuntime, err := ociruntime.NewStrictSupplementalGroups(cfg, logOutput, zlog.Logger.WithContext(context.TODO()))
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to initialize container runtime")
	}
	if err := containerRuntime.Exec(os.Args); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to run container runtime")
	}
}
