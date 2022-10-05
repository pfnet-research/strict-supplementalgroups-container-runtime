package bundle

import (
	"fmt"

	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/config"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	specFileName = "config.json"
)

type Bundle struct {
	Dir string

	spec   *specs.Spec
	logger zerolog.Logger
}
type ContainerInfo struct {
	PodNamespace  string
	PodName       string
	ContainerType string
	ContainerName string
}

func NewBundle(
	dir string,
) (*Bundle, error) {
	b := &Bundle{
		Dir:    dir,
		logger: zlog.With().Str("BundleDir", dir).Logger(),
	}
	if err := b.loadSpec(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Bundle) DoSpec(f func(s *specs.Spec) error) error {
	return f(b.spec)
}

func (b *Bundle) GetContainerInfo(
	logger zerolog.Logger,
	cfg *config.Config,
) (*ContainerInfo, error) {
	containerType, err := b.getContainerType(cfg)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to resolve container type in OCI Spec. Ignored.")
	}

	// resolve pod's namespace/name
	podNamespace, podName, err := b.getPodName(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve Pod in OCI Spec: %v", err)
	}

	containerName, err := b.getContainerName(cfg)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to resolve container name in OCI Spec. Ignored.")
	}

	return &ContainerInfo{
		ContainerType: containerType,
		PodNamespace:  podNamespace,
		PodName:       podName,
		ContainerName: containerName,
	}, nil
}

func (b *Bundle) getContainerType(cfg *config.Config) (string, error) {
	containerType, ok := b.spec.Annotations[cfg.ContainerTypeAnnotation]
	if !ok {
		return containerType, fmt.Errorf("%s annotation not found", cfg.ContainerTypeAnnotation)
	}
	return containerType, nil
}

func (b *Bundle) getPodName(cfg *config.Config) (string, string, error) {
	var podNamespace, podName string
	var ok bool

	podNamespace, ok = b.spec.Annotations[cfg.PodNamespaceAnnotation]
	if !ok || podNamespace == "" {
		return "", "", fmt.Errorf("%s annotation not found or empty", cfg.PodNamespaceAnnotation)
	}
	podName, ok = b.spec.Annotations[cfg.PodNameAnnotation]
	if !ok || podName == "" {
		return "", "", fmt.Errorf("%s annotation not found or empty", cfg.PodNameAnnotation)
	}

	return podNamespace, podName, nil
}

func (b *Bundle) getContainerName(cfg *config.Config) (string, error) {
	containerName, ok := b.spec.Annotations[cfg.ContainerNameAnnotation]
	if !ok {
		return "", fmt.Errorf("%s annotation not found", cfg.ContainerNameAnnotation)
	}
	return containerName, nil
}
