// Command amber runs amber scripts. Today it reports its version.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/radiospiel/amber/src/amber"
)

func main() {
	if err := newCommand(os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

// newCommand returns the amber command, writing to stdout, so a test can run
// it with SetArgs and read what it printed.
func newCommand(stdout io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "amber",
		Short:         "amber runs amber scripts",
		Version:       amber.Version(),
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.SetOut(stdout)
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print amber's version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), amber.Version())
			return err
		},
	})
	return root
}
