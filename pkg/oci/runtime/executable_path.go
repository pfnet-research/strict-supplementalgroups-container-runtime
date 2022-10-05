package runtime

import (
	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/lookup"
)

type executablePathRuntime struct {
	path string
}

func NewExecutablePathRuntime(path string) (Interface, error) {
	runtimePath, err := lookup.LookupExecutable(path)
	if err != nil {
		return nil, err
	}
	return &executablePathRuntime{
		path: runtimePath,
	}, nil
}

func (r *executablePathRuntime) Exec(args []string) error {
	execArgs := []string{r.path}
	if len(args) > 1 {
		execArgs = append(execArgs, args[1:]...)
	}
	return SyscallExecRuntime.Exec(execArgs)
}
