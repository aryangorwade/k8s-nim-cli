package get

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewGetCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get",
		Short:        "Display one or many NIM Operator custom resources.",
		Long:         `Prints a table of the most important information about the specified NIM Operator resources.`,
		Aliases:      []string{"list"},
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				fmt.Println(fmt.Errorf("unknown command(s) %q", strings.Join(args, " ")))
			}
			cmd.HelpFunc()(cmd, args)
		},
	}

	cmd.AddCommand(NewGetNIMCacheCommand(cmdFactory, streams))
	cmd.AddCommand(NewGetNIMServiceCommand(cmdFactory, streams))
	//cmd.AddCommand(NewGetNodesCommand(cmdFactory, streams))
	return cmd
}