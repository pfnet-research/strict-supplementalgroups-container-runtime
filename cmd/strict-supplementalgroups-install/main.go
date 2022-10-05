package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	cp "github.com/otiai10/copy"

	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/patch"
)

var (
	// injected in build time
	Version = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var hostPrefix = flag.String("hostPrefix", "/host", "install directory")
	var binDir = flag.String("bin-dir", "/opt/strict-supplementalgroups-container-runtime/bin", "directory containing binaries to install")
	var cri = flag.String("cri", "containerd", "CRI implementation (containerd or crio)")
	var criConfigPath = flag.String("cri-config", "", "CRI config file to add strict-supplementalgroups-container-runtime. Default is (hostPrefix)/etc/containerd/config.toml for containerd, (hostPrefix)/etc/crio/crio.conf for crio")
	var criConfigPatchPath = flag.String("cri-config-patch", "", "Patch file path for CRI config. If unspecified, default patch will be used.")
	var configPath = flag.String("config", "/etc/strict-supplementalgroups-container-runtime/config.toml", "config file to install for strict-supplementalgroups-container-runtime")
	flag.Parse()

	zlog.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: true}).With().Timestamp().Str("CRI", *cri).Logger()

	// resolve default crio config path
	if *criConfigPath == "" {
		switch strings.ToLower(*cri) {
		case "containerd":
			*criConfigPath = (*hostPrefix) + "/etc/containerd/config.toml"
		case "crio":
			*criConfigPath = (*hostPrefix) + "/etc/crio/crio.conf"
		default:
			zlog.Fatal().Str("version", Version).Str("cri", *cri).Msg("The CRI is not supported. Supported CRI is containerd or cri-o")
		}
	}

	zlog.Info().Str("version", Version).Msg("Starting strict-supplementalgroups-install")

	// install binaries
	fromDir := *binDir
	toDir := *hostPrefix + *binDir
	if err := cp.Copy(fromDir, toDir, cp.Options{
		OnSymlink: func(src string) cp.SymlinkAction { return cp.Deep },
	}); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to install binaries")
	}
	zlog.Info().Str("FromDir", fromDir).Str("ToDir", toDir).Msg("All binaries copied")

	// install config file
	fromFile := *configPath
	toDir = filepath.Join(*hostPrefix, "etc", "strict-supplementalgroups-container-runtime")
	toFile := filepath.Join(toDir, "config.toml")
	configData, err := os.ReadFile(fromFile)
	if err != nil {
		zlog.Fatal().Err(err).Str("From", fromFile).Str("To", toFile).Msg("Failed to copy config file")
	}
	if err := os.MkdirAll(toDir, 0755); err != nil {
		zlog.Fatal().Err(err).Str("From", fromFile).Str("To", toFile).Msg("Failed to copy config file")
	}
	err = os.WriteFile(toFile, configData, 0644)
	if err != nil {
		zlog.Fatal().Err(err).Str("From", fromFile).Str("To", toFile).Msg("Failed to copy config file")
	}
	zlog.Info().Str("From", fromFile).Str("To", toFile).Msg("Config file copied")

	// generate patch
	var criConfigPatch string
	if *criConfigPatchPath != "" {
		criConfigPatchRaw, err := os.ReadFile(*criConfigPatchPath)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Failed to read cri-config-patch")
		}
		criConfigPatch = string(criConfigPatchRaw)
		zlog.Info().Str("CRIConfigPatch", criConfigPatch).Msg("CRIConfigPatch loaded")
	} else {
		switch strings.ToLower(*cri) {
		case "containerd":
			criConfigPatch = containerdConfigPatch(*binDir + "/strict-supplementalgroups-container-runtime")
		case "crio":
			criConfigPatch = crioConfigPatch(*binDir + "/strict-supplementalgroups-container-runtime")
		default:
			zlog.Fatal().Str("cri", *cri).Msg("The CRI is not supported. Supported CRI is containerd or cri-o")
		}
		zlog.Info().Str("CRIConfigPatch", criConfigPatch).Msg("CRIConfigPatch generated")
	}

	// apply the generated patch
	updated, err := updateCriConfig(strings.ToLower(*cri), *criConfigPath, criConfigPatch)
	if err != nil {
		zlog.Fatal().Err(err).Str("CRIConfigPath", *criConfigPath).Msg("Failed to update CRI config")
	}
	if !updated {
		zlog.Info().Str("CRIConfigPath", *criConfigPath).Msg("CRI Config is already patched")
		zlog.Info().Msg("Installation Finished")
	} else {
		zlog.Info().Str("CRIConfigPath", *criConfigPath).Msg("CRIConfig updated")
	}

	if err := reloadCRI(strings.ToLower(*cri)); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to reload CRI")
	}
	zlog.Info().Msg("CRI reloaded")
	if strings.ToLower(*cri) == "crio" {
		zlog.Warn().Msg("You need to restart crio manually because crio does not support reloading runtime configuration (see: https://github.com/cri-o/cri-o/issues/6036)")
	}
	zlog.Info().Msg("Installation Finished")
	<-ctx.Done()
}

func updateCriConfig(
	criName, criConfigPath, criConfigPatch string,
) (bool, error) {
	configRaw, err := os.ReadFile(criConfigPath)
	if err != nil {
		return false, fmt.Errorf("Failed to open %s: %w", criConfigPath, err)
	}
	patched, err := patch.TOML(string(configRaw), []string{criConfigPatch}, []string{})
	if err != nil {
		return false, fmt.Errorf("Failed to patch %s config: %w", criName, err)
	}
	if string(configRaw) == patched {
		return false, nil
	}
	if err := os.WriteFile(criConfigPath, []byte(patched), 0644); err != nil {
		return false, err
	}

	return true, nil
}

func containerdConfigPatch(
	binaryPath string,
) string {
	return heredoc.Docf(`
		[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.strict-supplementalgroups]
		runtime_type = "io.containerd.runc.v2"
		[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.strict-supplementalgroups.options]
		BinaryName = "%s"
		`, binaryPath)
}

func crioConfigPatch(
	binaryPath string,
) string {
	return heredoc.Docf(`
	[crio.runtime.runtimes.strict-supplementalgroups]
	runtime_path = "%s"
	`, binaryPath)
}

func reloadCRI(criName string) error {
	out, err := runCmd("pgrep", "--exact", criName)
	if err != nil {
		return err
	}
	_, err = runCmd("kill", "-SIGHUP", strings.TrimSpace(string(out)))
	if err != nil {
		return err
	}
	return nil
}

func runCmd(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			zlog.Error().Err(err).
				Bytes("Stdout", out).
				Bytes("Stderr", ee.Stderr).
				Strs("Command", append([]string{name}, args...)).
				Msg("Command Failed")
		} else {
			zlog.Error().Err(err).
				Strs("Command", append([]string{name}, args...)).
				Msg("Command Failed")
		}
		return out, err
	}
	return out, nil
}
