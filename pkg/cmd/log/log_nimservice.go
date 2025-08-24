package log

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"k8s-nim-operator-cli/pkg/util/client"
	"k8s-nim-operator-cli/pkg/util"
	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

func NewLogNIMServiceCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := util.NewFetchResourceOptions(cmdFactory, streams)

	cmd := &cobra.Command{
		Use:               "nimservice NAME",
		Aliases:           []string{""},
		Short:             "Get NIMService logs.",
		SilenceUsage:      true,
		// ValidArgsFunction: completion.RayClusterCompletionFunc(cmdFactory),
		Args:              cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.CompleteNamespace(args, cmd); err != nil {
				return err
			}
			// running cmd.Execute or cmd.ExecuteE sets the context, which will be done by root
			k8sClient, err := client.NewClient(cmdFactory)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			return Run(cmd.Context(), options, k8sClient, appsv1alpha1.NIMServiceList{})
		},
	}
	cmd.Flags().BoolVarP(&options.AllNamespaces, "all-namespaces", "A", false, "If present, list the requested NIMService's logs across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	return cmd
}

func printNIMServices(nimServiceList *appsv1alpha1.NIMServiceList, output io.Writer) error {
	resultTablePrinter := printers.NewTablePrinter(printers.PrintOptions{})

	resTable := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "Name", Type: "string"},
			{Name: "Namespace", Type: "string"},
			{Name: "Image", Type: "string"},
			{Name: "Expose Service", Type: "string"},
			{Name: "Replicas", Type: "int"},
			{Name: "Scale", Type: "string"}, // if enabled, shows HPA maxReplicas / minReplicas
			{Name: "Storage", Type: "string"}, // the kind and if pvc the pvc details
			{Name: "Resources", Type: "string"}, // if any limits/resquests/claims shows them here
			{Name: "State", Type: "string"},// only status
			{Name: "Age", Type: "string"},
		},
	}

	for _, nimservice := range nimServiceList.Items {
		age := duration.HumanDuration(time.Since(nimservice.GetCreationTimestamp().Time))
		if nimservice.GetCreationTimestamp().Time.IsZero() {
			age = "<unknown>"
		}

		resTable.Rows = append(resTable.Rows, v1.TableRow{
			Cells: []interface{}{
				nimservice.GetName(),
				nimservice.GetNamespace(), 
				fmt.Sprintf("%s %s", nimservice.Spec.Image.Repository, nimservice.Spec.Image.Tag),
				getExpose(&nimservice),
				nimservice.Spec.Replicas,
				getScale(&nimservice),
				getStorage(&nimservice),
				getNIMServiceResources(&nimservice),
				nimservice.Status.State, 
				age,
			},
		})
	}

	return resultTablePrinter.PrintObj(resTable, output)
}