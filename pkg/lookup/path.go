package lookup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	envPath = "PATH"
)

var (
	defaultPath   = []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"}
	pathSeparator = ":"
)

func LookupExecutable(path string) (string, error) {
	if os.Getenv(envPath) == "" {
		if err := os.Setenv(envPath, strings.Join(defaultPath, pathSeparator)); err != nil {
			return "", fmt.Errorf("Failed to set PATH environment variable: %v", err)
		}
	}

	executablePath, err := exec.LookPath(path)
	if err != nil {
		return "", fmt.Errorf("Not found executable for %s: %v", path, err)
	}

	return executablePath, nil
}
