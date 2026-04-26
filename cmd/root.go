package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build metadata. Overridden via -ldflags="-X .../cmd.Version=..." by GoReleaser.
// When unset (e.g. `go run`, `go build` without ldflags), we fall back to the
// embedded VCS info from the Go build, then to "dev".
var (
	Version   = ""
	Commit    = ""
	BuildDate = ""
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "healthd",
		Short:   "Host health-check daemon",
		Version: versionString(),
	}
	root.SetVersionTemplate("{{.Version}}\n")

	root.AddCommand(newCheckCommand())
	root.AddCommand(newInitCommand())
	root.AddCommand(newNotifyCommand())
	root.AddCommand(newRunCommand())
	root.AddCommand(newStatusCommand())
	root.AddCommand(newValidateCommand())
	return root
}

func versionString() string {
	v, c, d := Version, Commit, BuildDate
	if v == "" || c == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					if c == "" {
						c = s.Value
					}
				case "vcs.time":
					if d == "" {
						d = s.Value
					}
				}
			}
		}
	}
	if v == "" {
		v = "dev"
	}
	if c == "" {
		return v
	}
	short := c
	if len(short) > 7 {
		short = short[:7]
	}
	if d == "" {
		return fmt.Sprintf("%s (%s)", v, short)
	}
	return fmt.Sprintf("%s (%s, %s)", v, short, d)
}
