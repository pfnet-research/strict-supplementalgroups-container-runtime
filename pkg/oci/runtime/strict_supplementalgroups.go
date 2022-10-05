package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	corev1 "k8s.io/api/core/v1"

	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/config"
	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/kubelet"
	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/lookup"
	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/oci/bundle"
)

type GidSet map[int64]struct{}

type strictSupplementalGroupsRuntime struct {
	cfg           *config.Config
	kubeletClient *kubelet.Client

	runtimeLogWriter io.Writer
	runtimeLogCtx    context.Context

	underlyingRuntime Interface
}

func NewStrictSupplementalGroups(
	cfg *config.Config,
	runtimeLogWriter io.Writer,
	runtimeLogCtx context.Context,
) (Interface, error) {
	kubeletClient, err := kubelet.NewKubeletClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to create kubelet client: %v", err)
	}

	underlyingRuntime, err := NewExecutablePathRuntime(cfg.Runtime)
	if err != nil {
		return nil, err
	}

	return &strictSupplementalGroupsRuntime{
		cfg:           cfg,
		kubeletClient: kubeletClient,

		runtimeLogWriter: runtimeLogWriter,
		runtimeLogCtx:    runtimeLogCtx,

		underlyingRuntime: underlyingRuntime,
	}, nil
}

func (r *strictSupplementalGroupsRuntime) Exec(args []string) error {
	zlog.Debug().Strs("Command", args).Msg("Container runtime command invoked")

	// parse command line arguments
	var crArgs *RuntimeArgs
	{
		var err error
		crArgs, err = GetRuntimeArgs(args)
		if err != nil {
			return fmt.Errorf("Fail to get oci runtime args: %w", err)
		}
		zlog.Debug().Interface("Args", crArgs).Msg("Container runtime command line arguments parsed")
	}

	clogWriter, closeContainerLogFile, err := r.createContainerLogWriter(crArgs)
	if err != nil {
		return fmt.Errorf("Failed to create container Logger: %w", err)
	}
	defer closeContainerLogFile()

	// this logger outputs both runtime log(specified in config) and container log(specified in runtime command)
	logger := zerolog.Ctx(r.runtimeLogCtx).Output(
		io.MultiWriter(r.runtimeLogWriter, clogWriter),
	).With().Str("ContainerId", crArgs.ContainerId).Str("Command", string(crArgs.Command)).Logger()

	// validate OCI spec only when "create", "start", "exec" bundle command
	if err := func() error {
		switch crArgs.Command {
		case CommandCreate:
			return r.enforceSupplementalGroupsOnCreate(logger, crArgs)
		case CommandStart:
			return r.enforceSupplementalGroupsOnStart(logger, crArgs)
		case CommandExec:
			return r.enforceSupplementalGroupsOnExecute(logger, crArgs)
		default:
			// NOP
			logger.Info().Strs("Command", args).Msg("Ignored the invocation")
			return nil
		}
	}(); err != nil {
		zlog.Error().Err(err).Msg("Failed to enforce SupplementalGroups")
		return err
	}

	return r.underlyingRuntime.Exec(args)
}

func (r *strictSupplementalGroupsRuntime) enforceSupplementalGroupsOnCreate(logger zerolog.Logger, crArgs *RuntimeArgs) error {
	b, err := bundle.NewBundle(crArgs.Options.Bundle)
	if err != nil {
		return fmt.Errorf("Fail to load OCI bundle: %w", err)
	}
	logger = logger.With().Str("BundleDir", b.Dir).Logger()
	return r.enforceSupplementalGroupsOnBundle(logger, b)
}

func (r *strictSupplementalGroupsRuntime) enforceSupplementalGroupsOnStart(logger zerolog.Logger, crArgs *RuntimeArgs) error {
	// find bundle from coantainerId
	b, err := r.getBundleForContainer(crArgs.Options.Root, crArgs.ContainerId)
	if err != nil {
		return fmt.Errorf("Failed to find bundle for containerId %s: %v", crArgs.ContainerId, err)
	}
	logger = logger.With().Str("BundleDir", b.Dir).Logger()
	return r.enforceSupplementalGroupsOnBundle(logger, b)
}

func (r *strictSupplementalGroupsRuntime) enforceSupplementalGroupsOnExecute(logger zerolog.Logger, crArgs *RuntimeArgs) error {
	// find bundle from coantainerId
	b, err := r.getBundleForContainer(crArgs.Options.Root, crArgs.ContainerId)
	if err != nil {
		return fmt.Errorf("Failed to find bundle for containerId %s: %v", crArgs.ContainerId, err)
	}
	logger = logger.With().Str("BundleDir", b.Dir).Logger()

	// resolve pod's namespace/name and container and its container type(sandbox, container)
	ctrInfo, err := b.GetContainerInfo(logger, r.cfg)
	if err != nil {
		return fmt.Errorf("Failed to resolve container info from OCI bundle: %w", err)
	}
	logger = logger.With().
		Str("ContainerType", ctrInfo.ContainerType).
		Str("Pod", ctrInfo.PodNamespace+"/"+ctrInfo.PodName).
		Str("Container", ctrInfo.ContainerName).
		Logger()
	logger.Info().Msg("Container info loaded")

	// no need to enforce supplementalGroups because sandbox is not a user container.
	if ctrInfo.ContainerType == "sandbox" {
		logger.Info().Msg("Skip to enforce supplementalGroups for sandbox containers")
		return nil
	}

	// read process spec
	processRaw, err := ioutil.ReadFile(crArgs.Options.Process)
	if err != nil {
		return fmt.Errorf("Failed to read process file %s: %v", crArgs.Options.Process, err)
	}
	var process specs.Process
	err = json.Unmarshal(processRaw, &process)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal process file %s: %v", crArgs.Options.Process, err)
	}
	logger.Debug().Interface("Process", process).Msg("Process spec is parsed")

	pod, err := r.kubeletClient.Pod(ctrInfo.PodNamespace, ctrInfo.PodName)
	if err != nil {
		return fmt.Errorf("Failed to get pod: %v", err)
	}

	enforced := r.enforceSupplementalGroupsOnProcessSpec(logger, &process, pod)
	if enforced {
		jsonRaw, err := json.Marshal(&process)
		if err != nil {
			return fmt.Errorf("Failed to marshal process spec: %w", err)
		}
		if err := ioutil.WriteFile(crArgs.Options.Process, jsonRaw, 0644); err != nil {
			return fmt.Errorf("Failed to update process spec: %w", err)
		}
		logger.Info().Msg("SupplementalGroups enforced successfully")
		return nil
	}
	return nil
}
func (r *strictSupplementalGroupsRuntime) getBundleForContainer(root, containerId string) (*bundle.Bundle, error) {
	runtime, err := lookup.LookupExecutable(r.cfg.Runtime)
	if err != nil {
		return nil, fmt.Errorf("Failed to find runtime: %v", err)
	}

	command := []string{runtime, "--root", root, "state", containerId}
	cmd := exec.Command(command[0], command[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if err != nil {
		zlog.Error().Err(err).Str("Stdout", stdout.String()).Str("Stderr", stderr.String()).Strs("Command", command).Msg("Failed to execute Command")
		return nil, fmt.Errorf("Failed to execute command '%s': %v", strings.Join(command, " "), err)
	}

	stateRaw := stdout.Bytes()
	var state specs.State
	if err := json.Unmarshal(stateRaw, &state); err != nil {
		return nil, fmt.Errorf("Failed to parse state json: %v", err)
	}
	b, err := bundle.NewBundle(state.Bundle)
	if err != nil {
		return nil, fmt.Errorf("Fail to load OCI bundle: %v", err)
	}
	return b, nil
}

func (r *strictSupplementalGroupsRuntime) createContainerLogWriter(crArgs *RuntimeArgs) (io.Writer, func() error, error) {
	containerLogFile := crArgs.Options.Log
	if containerLogFile == "" {
		// setup container log (if specified)
		containerLogFile = "/dev/null"
	}
	containerLogWriter, err := os.OpenFile(filepath.Join(containerLogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("Failed to open container log file: %w", err)
	}
	var containerLogOutput io.Writer = containerLogWriter
	if crArgs.Options.LogFormat == "text" {
		containerLogOutput = zerolog.ConsoleWriter{Out: containerLogWriter, NoColor: true, TimeFormat: time.RFC3339}
	}
	return containerLogOutput, containerLogWriter.Close, nil
}

func (r *strictSupplementalGroupsRuntime) enforceSupplementalGroupsOnBundle(
	logger zerolog.Logger,
	b *bundle.Bundle,
) error {
	// resolve pod's namespace/name and container and its container type(sandbox, container)
	ctrInfo, err := b.GetContainerInfo(logger, r.cfg)
	if err != nil {
		return fmt.Errorf("Failed to resolve container info from OCI bundle: %w", err)
	}
	logger = logger.With().
		Str("ContainerType", ctrInfo.ContainerType).
		Str("Pod", ctrInfo.PodNamespace+"/"+ctrInfo.PodName).
		Str("Container", ctrInfo.ContainerName).
		Logger()
	logger.Info().Msg("Container info loaded")

	// no need to enforce supplementalGroups because sandbox is not a user container.
	if ctrInfo.ContainerType == "sandbox" {
		logger.Info().Msg("Skip to enforce supplementalGroups for sandbox containers")
		return nil
	}

	pod, err := r.kubeletClient.Pod(ctrInfo.PodNamespace, ctrInfo.PodName)
	if err != nil {
		return fmt.Errorf("Failed to get pod: %v", err)
	}

	var enforced bool
	_ = b.DoSpec(func(s *specs.Spec) error {
		if s.Process == nil {
			s.Process = &specs.Process{}
		}
		enforced = r.enforceSupplementalGroupsOnProcessSpec(logger, s.Process, pod)
		return nil
	})
	if enforced {
		if err := b.SaveSpec(); err != nil {
			return fmt.Errorf("Failed to update OCI bundle: %w", err)
		}
		logger.Info().Msg("SupplementalGroups enforced successfully")
		return nil
	}
	return nil
}

func (r *strictSupplementalGroupsRuntime) enforceSupplementalGroupsOnProcessSpec(
	logger zerolog.Logger,
	processSpec *specs.Process,
	pod *corev1.Pod,
) bool /* enforcement performed or not*/ {
	// get additionalGids and supplementalGroups
	additionalGids := r.getAdditionalGids(processSpec)
	logger.Debug().Interface("additionalGids", additionalGids).Msg("Additional Gids loaded")

	supplementalGroups, fsGroup := r.getSupplementalGroupsAndFsGroup(pod)
	logger.Debug().Interface("supplementalGroups", supplementalGroups).Interface("fsGroup", fsGroup).Msg("Supplemental Groups And FsGroup loaded")
	allowedGids := GidSet{}
	for k := range supplementalGroups {
		allowedGids[k] = struct{}{}
	}
	if fsGroup != nil {
		allowedGids[*fsGroup] = struct{}{}
	}

	// it must satisfies additionalGids ⊆ (supplementalGroups ∪ fsGroup)
	violatedGids := []int64{}
	enforcedGids := []uint32{}
	for k := range additionalGids {
		if _, ok := allowedGids[k]; ok {
			enforcedGids = append(enforcedGids, uint32(k))
		} else {
			violatedGids = append(violatedGids, k)
		}
	}
	if len(violatedGids) > 0 {
		logger.Info().
			Ints64("violatedGids", violatedGids).
			Interface("supplementalGroups", supplementalGroups).
			Interface("fsGroups", fsGroup).
			Interface("additionalGids", additionalGids).
			Interface("enforcedGids", enforcedGids).
			Msg("Detected violated gids such that it is in additionalGroups but not in (supplementalGroups ∪ fsGroup). Dropping violated Gids")
		processSpec.User.AdditionalGids = enforcedGids
		return true
	}
	logger.Info().
		Interface("supplementalGroups", supplementalGroups).
		Interface("fsGroups", fsGroup).
		Interface("additionalGids", additionalGids).
		Msg("No need to replace additionalGids")

	return false
}

func (r *strictSupplementalGroupsRuntime) getAdditionalGids(process *specs.Process) GidSet {
	additionalGids := GidSet{}
	if process == nil {
		return additionalGids
	}

	for _, gid := range process.User.AdditionalGids {
		additionalGids[int64(gid)] = struct{}{}
	}

	return additionalGids
}

func (r *strictSupplementalGroupsRuntime) getSupplementalGroupsAndFsGroup(pod *corev1.Pod) (GidSet, *int64) {
	supplementalGroups := GidSet{}
	if pod.Spec.SecurityContext == nil {
		return supplementalGroups, nil
	}

	for _, gid := range pod.Spec.SecurityContext.SupplementalGroups {
		supplementalGroups[gid] = struct{}{}
	}

	return supplementalGroups, pod.Spec.SecurityContext.FSGroup
}
