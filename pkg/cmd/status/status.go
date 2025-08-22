package status

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewStatusCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Describe the status of a NIM Operator custom resource.",
		Long:         `Prints a table about the status of the specified NIM Operator resource.`,
		Aliases:      []string{},
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				fmt.Println(fmt.Errorf("unknown command(s) %q", strings.Join(args, " ")))
			}
			cmd.HelpFunc()(cmd, args)
		},
	}

	cmd.AddCommand(NewStatusNIMCacheCommand(cmdFactory, streams))
	cmd.AddCommand(NewStatusNIMServiceCommand(cmdFactory, streams))
	return cmd
}