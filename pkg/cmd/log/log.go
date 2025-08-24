package log

import (
	"context"
	"fmt"
	"strings"

	"k8s-nim-operator-cli/pkg/util"
	"k8s-nim-operator-cli/pkg/util/client"

	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewGetCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := util.NewFetchResourceOptions(cmdFactory, streams)

	cmd := &cobra.Command{
		Use:          "log RESOURCE NAME",
		Short:        "Display the logs of a specified NIM Operator custom resource.",
		Aliases:      []string{"logs"},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				// Show help if no args provided.
				cmd.HelpFunc()(cmd, args)
			case 2:
				// Proceed as normal if two args provided.
				if err := options.CompleteNamespace(args, cmd); err != nil {
					return err
				}
				// TODO: rewrite this to print out log or return error. currently incorrect.
				// running cmd.Execute or cmd.ExecuteE sets the context, which will be done by root
				k8sClient, err := client.NewClient(cmdFactory)
				if err != nil {
					return fmt.Errorf("failed to create client: %w", err)
				}
				return Run(cmd.Context(), options, k8sClient, appsv1alpha1.NIMServiceList{})	
			default:
				fmt.Println(fmt.Errorf("unknown command(s) %q", strings.Join(args, " ")))
			}
		},
	}

	cmd.AddCommand(NewLogNIMServiceCommand(cmdFactory, streams))
	cmd.AddCommand(NewLogNIMCacheCommand(cmdFactory, streams))
	return cmd
}

func Run(ctx context.Context, options *util.FetchResourceOptions, k8sClient client.Client, resourceListType interface{}) error {
	resourceList, err := util.FetchResources(ctx, options, k8sClient, resourceListType)
	if err != nil {
		return err
	}

	switch resourceListType.(type) {

	case appsv1alpha1.NIMServiceList:
		// Cast resourceList to NIMServiceList.
		nimServiceList, ok := resourceList.(*appsv1alpha1.NIMServiceList)
		if !ok {
			return fmt.Errorf("failed to cast resourceList to NIMServiceList")
		}
		return printNIMServices(nimServiceList, options.IoStreams.Out)

	case appsv1alpha1.NIMCacheList:
		// Cast resourceList to NIMCacheList.
		nimCacheList, ok := resourceList.(*appsv1alpha1.NIMCacheList)
		if !ok {
			return fmt.Errorf("failed to cast resourceList to NIMCacheList")
		}
		return printNIMCaches(nimCacheList, options.IoStreams.Out)
	}

	return err
}
