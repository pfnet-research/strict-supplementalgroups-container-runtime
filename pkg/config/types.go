package config

import (
	"github.com/rs/zerolog"
)

// Config is the data structucture for
// the configuration of strict-supplementalgroups-container-runtime
type Config struct {
	// Runtime is the low-level container runtime binary path of strict-supplementalgroups-container-runtime
	Runtime string `toml:"runtime" default:"runc"`

	// KubeletUrl is the kubelet's HTTP endpoint
	KubeletUrl string `toml:"kubelet-url" default:"https://127.0.0.1:10250"`

	// KubeConfig is the kubeconfig file path to access to KubeletUrl
	KubeConfig string `toml:"kubeconfig" default:"/etc/kubernetes/kubelet.conf"`

	// PodNamespaceAnnotation is the annotation key in OCI container spec (config.json) representing pod's namespace.
	// The annotation key depends on CRI(Container Runtime Interface) implementations.  The default value is containerd's.
	PodNamespaceAnnotation string `toml:"pod-namespace-annotation" default:"io.kubernetes.cri.sandbox-namespace"`

	// PodNameAnnotation is the annotation key in OCI container spec (config.json) representing pod's name.
	// The annotation key depends on CRI(Container Runtime Interface) implementations.  The default value is containerd's.
	PodNameAnnotation string `toml:"pod-name-annotation" default:"io.kubernetes.cri.sandbox-name"`

	// ContainerNameAnnotation the annotation key in OCI container spec (config.json) representing pod's name.
	// The annotation key depends on CRI(Container Runtime Interface) implementations.  The default value is containerd's.
	ContainerNameAnnotation string `toml:"container-name-annotation" default:"io.kubernetes.cri.container-name"`

	// ContainerTypeAnnotaiton is the annotation key in OCI container spec (config.json) representing container type (sandbox or container)
	// The annotation key depends on CRI(Container Runtime Interface) implementations.  The default value is containerd's.
	ContainerTypeAnnotation string `toml:"container-type-annotation" default:"io.kubernetes.cri.container-type"`

	// Logging is configuration for logging
	Logging LogConfig `toml:"logging"`
}

type LogConfig struct {
	// LogFile is the file path to strict-supplementalgroups-container-runtime's log
	LogFile string `toml:"log-file" defaults:"/dev/null"`

	// LogLevelStr is the log level of strict-supplementalgroups-container-runtime
	LogLevelStr string `toml:"log-level" default:"info"`

	// LogFormat is format of the log (text or json). json is default.
	LogFormat string `toml:"log-format" default:"json"`

	// Rotation is configuration for log file rotation
	Rotation LogRotationConfig `toml:"rotation"`

	// below fields are filled when loading
	LogLevel zerolog.Level `toml:"-"`
}

type LogRotationConfig struct {
	// MaxSizeInMB is he maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxSizeInMB int `toml:"max-size-in-mb" default:"100"`

	// MaxBackups is the maximum number of old log files to retain.  The default
	// is to retain all old log files (though MaxAge may still cause them to get
	// deleted.)
	MaxBackups int `toml:"max-backups" default:"0"`

	// MaxAgeInDay is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename.  Note that a day is defined as 24
	// hours and may not exactly correspond to calendar days due to daylight
	// savings, leap seconds, etc. The default is not to remove old log files
	// based on age.
	MaxAgeInDay int `toml:"max-age-in-day" default:"30"`

	// Compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool `toml:"compress" default:"false"`
}
