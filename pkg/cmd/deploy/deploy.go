package deploy

// TODO: delete the old methods for streaming pod logs in fetch_resource.go and in  log.go.
// TODO: implement autocompletion
// TODO: remove unnecessayr packages like raycluster from go.sum/mod

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewDeployCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "deploy",
		Short:        "Deploy a NIM Operator custom resource",
		Long:         `Deploys the specified NIM Operator resource with the parameters specified through flags.`,
		Aliases:      []string{"create"},
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				fmt.Println(fmt.Errorf("unknown command(s) %q", strings.Join(args, " ")))
			}
			cmd.HelpFunc()(cmd, args)
		},
	}

	// cmd.AddCommand(NewDeployNIMCacheCommand(cmdFactory, streams))
	cmd.AddCommand(NewDeployNIMServiceCommand(cmdFactory, streams))
	return cmd
}