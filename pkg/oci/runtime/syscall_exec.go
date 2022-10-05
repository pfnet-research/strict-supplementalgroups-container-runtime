package runtime

import (
	"fmt"
	"os"
	"syscall"

	zlog "github.com/rs/zerolog/log"
)

var (
	SyscallExecRuntime = &syscallExecRuntime{}
)

type syscallExecRuntime struct{}

func (s *syscallExecRuntime) Exec(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("Could not exec empty args")
	}

	zlog.Trace().Strs("Command", args).Msg("Executing execve(2) system call")
	err := syscall.Exec(args[0], args, os.Environ())
	if err != nil {
		return fmt.Errorf("Could not exec '%v': %v", args[0], err)
	}

	return fmt.Errorf("Unexpected return from exec '%v': %v", args[0], err)
}
