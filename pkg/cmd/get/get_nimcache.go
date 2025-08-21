package get

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
	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

func NewGetNIMCacheCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := NewGetResourceOptions(cmdFactory, streams)

	cmd := &cobra.Command{
		Use:               "nimcache [NAME]",
		Aliases:           []string{"nimcaches"},
		Short:             "Get NIMCache information.",
		SilenceUsage:      true,
		// ValidArgsFunction: completion.RayClusterCompletionFunc(cmdFactory),
		Args:              cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.CompleteNamespace(args, cmd); err != nil {
				return err
			}
			// running cmd.Execute or cmd.ExecuteE sets the context, which will be done by root
			k8sClient, err := client.NewClient(cmdFactory)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			return options.Run(cmd.Context(), k8sClient, &appsv1alpha1.NIMCacheList{})
		},
	}
	cmd.Flags().BoolVarP(&options.allNamespaces, "all-namespaces", "A", false, "If present, list the requested NIMCaches across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	return cmd
}

func printNIMCaches(nimCacheList *appsv1alpha1.NIMCacheList, output io.Writer) error {
	resultTablePrinter := printers.NewTablePrinter(printers.PrintOptions{})

	resTable := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "Name", Type: "string"},
			{Name: "Namespace", Type: "string"},
			{Name: "Source", Type: "string"},
			{Name: "Model/ModelPuller", Type: "string"},
			{Name: "CPU", Type: "string"},
			{Name: "Memory", Type: "string"},
			{Name: "PVC Volume", Type: "string"},
			{Name: "State", Type: "string"},
			{Name: "Age", Type: "string"},
		},
	}

	for _, nimcache := range nimCacheList.Items {
		age := duration.HumanDuration(time.Since(nimcache.GetCreationTimestamp().Time))
		if nimcache.GetCreationTimestamp().Time.IsZero() {
			age = "<unknown>"
		}

		resTable.Rows = append(resTable.Rows, v1.TableRow{
			Cells: []interface{}{
				nimcache.GetName(),
				nimcache.GetNamespace(), 
				getSource(&nimcache),
				getModel(&nimcache),
				nimcache.Spec.Resources.CPU.String(), 
				nimcache.Spec.Resources.Memory.String(),
				getPVCDetails(&nimcache),
				nimcache.Status.State,
				age,
			},
		})
	}

	return resultTablePrinter.PrintObj(resTable, output)
}

// Return source.
func getSource(nimCache *appsv1alpha1.NIMCache) string {
	if nimCache.Spec.Source.NGC != nil {
		return "NGC"
	} else if nimCache.Spec.Source.DataStore != nil {
		return "NVIDIA NeMo DataStore"
	} 
	return "HuggingFace Hub"
}

// Return either ModelPuller or ModelName/Endpoint.
// nimCache.Spec.Source.HF undefined (type "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1".NIMSource has no field or method HF).
func getModel(nimCache *appsv1alpha1.NIMCache) string {
	if nimCache.Spec.Source.NGC != nil {
		return nimCache.Spec.Source.NGC.ModelPuller
	} else {
		if nimCache.Spec.Source.DataStore.ModelName == nil {
			return nimCache.Spec.Source.DataStore.Endpoint
		}
		return *nimCache.Spec.Source.DataStore.ModelName
	}

	/*
	if nimCache.Spec.Source.HF != nil {
		if nimCache.Spec.Source.HF.ModelName == nil {
			return nimCache.Spec.Source.HF.Endpoint
		}
		return *nimCache.Spec.Source.HF.ModelName
	}
	*/
}

func getPVCDetails(nimCache *appsv1alpha1.NIMCache) string {
	return fmt.Sprintf("%s, %s", nimCache.Spec.Storage.PVC.Name, nimCache.Spec.Storage.PVC.Size)
}