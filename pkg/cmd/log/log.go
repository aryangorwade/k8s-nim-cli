package log

import (
	"context"
	"fmt"
	"strings"

	"k8s-nim-operator-cli/pkg/util"
	"k8s-nim-operator-cli/pkg/util/client"

	// appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewLogCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := util.NewFetchResourceOptions(cmdFactory, streams)

	cmd := &cobra.Command{
		Use:          "log RESOURCE NAME",
		Short:        "Display the logs of a specified NIM Operator custom resource.",
		Long: 
		`Display logs for NIM Operator custom resources.

		Supported RESOURCE types:
		  - nimcache
		  - nimservice
		
		Examples:
		  # Show logs for a NIMCache resource named "my-cache"
		  mycli log nimcache my-cache
		
		  # Show logs for a NIMService resource named "my-service"
		  mycli log nimservice my-service`,
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
				// running cmd.Execute or cmd.ExecuteE sets the context, which will be done by root
				k8sClient, err := client.NewClient(cmdFactory)
				if err != nil {
					return fmt.Errorf("failed to create client: %w", err)
				}
				return Run(cmd.Context(), options, k8sClient)	
			default:
				fmt.Println(fmt.Errorf("unknown command(s) %q", strings.Join(args, " ")))
			}
			return nil
		},
	}
	
	return cmd
}

func Run(ctx context.Context, options *util.FetchResourceOptions, k8sClient client.Client) error {
	/*
	resourceList, err := util.FetchResources(ctx, options, k8sClient)
	if err != nil {
		return err
	}
	*/
	switch options.ResourceType {

	case util.NIMService:
		// Cast resourceList to NIMServiceList.
		/*
		nimServiceList, ok := resourceList.(*appsv1alpha1.NIMServiceList)
		if !ok {
			return fmt.Errorf("failed to cast resourceList to NIMServiceList")
		}
		return printNIMServices(nimServiceList, options.IoStreams.Out)
		*/ 
		return nil

	case util.NIMCache:
		/*
		// Cast resourceList to NIMCacheList.
		nimCacheList, ok := resourceList.(*appsv1alpha1.NIMCacheList)
		if !ok {
			return fmt.Errorf("failed to cast resourceList to NIMCacheList")
		}
		return printNIMCaches(nimCacheList, options.IoStreams.Out)
		*/ 
		return nil
	}

	// return err
	return nil
}
