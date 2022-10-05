package runtime

import (
	"strings"

	"github.com/jessevdk/go-flags"
)

type Command string

var (
	CommandCreate Command = "create"
	CommandStart  Command = "start"
	CommandExec   Command = "exec"
)

type RuntimeArgs struct {
	Command     Command
	ContainerId string
	Options     RuntimeOpts
}

type RuntimeOpts struct {
	// global flags
	Root      string `long:"root"`
	Log       string `long:"log"`
	LogFormat string `long:"log-format"`

	// common flags for create/exec
	PidFile string `long:"pid-file"`

	// flags for create
	Bundle string `long:"bundle" short:"b"`

	// flags for exec
	Process string `long:"process" short:"p"`
}

// GetRuntimeArgs analyze command line arguments passed from OCI to low level container runtime
// and returns analysis result for later use
func GetRuntimeArgs(args []string) (*RuntimeArgs, error) {
	runtimeOpts := RuntimeOpts{}
	parser := flags.NewParser(&runtimeOpts, flags.IgnoreUnknown)

	remainingArgs, err := parser.ParseArgs(args)
	if err != nil {
		return nil, err
	}
	runtimeArgs := RuntimeArgs{Options: runtimeOpts}

	nonOptionArguments := []string{}
	for _, r := range remainingArgs {
		if !strings.HasPrefix(r, "-") {
			nonOptionArguments = append(nonOptionArguments, r)
		}
	}
	if len(nonOptionArguments) >= 3 {
		// argument[0] is strict-supplementalgroups-container-runtime itself

		// command should come at first
		runtimeArgs.Command = Command(nonOptionArguments[1])

		// container-id should come at the last
		runtimeArgs.ContainerId = nonOptionArguments[len(nonOptionArguments)-1]
	}

	return &runtimeArgs, nil
}
