package status

import (
	"fmt"
	"io"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"k8s-nim-operator-cli/pkg/util/client"

	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

func NewStatusNIMCacheCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := NewStatusResourceOptions(cmdFactory, streams)

	cmd := &cobra.Command{
		Use:          "nimcache [NAME]",
		Aliases:      []string{"nimcaches"},
		Short:        "Get NIMCache information.",
		SilenceUsage: true,
		// ValidArgsFunction: completion.RayClusterCompletionFunc(cmdFactory),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.CompleteNamespace(args, cmd); err != nil {
				return err
			}
			// running cmd.Execute or cmd.ExecuteE sets the context, which will be done by root
			k8sClient, err := client.NewClient(cmdFactory)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			return options.Run(cmd.Context(), k8sClient, appsv1alpha1.NIMCacheList{})
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
			{Name: "State", Type: "string"},
			{Name: "PVC", Type: "string"},
			{Name: "Type/Status", Type: "int"},
			{Name: "Last Transition Time", Type: "string"},
			{Name: "Message", Type: "string"},
			{Name: "Age", Type: "string"},
		},
	}

	for _, nimcache := range nimCacheList.Items {
		age := duration.HumanDuration(time.Since(nimcache.GetCreationTimestamp().Time))
		if nimcache.GetCreationTimestamp().Time.IsZero() {
			age = "<unknown>"
		}

		cond := apimeta.FindStatusCondition(nimcache.Status.Conditions, "Ready")
		if cond == nil {
			return fmt.Errorf("The Ready condition is not set yet.")
		}

		// Determine which condition to use for Message and LastTransitionTime.
		msgCond := cond
		// Prefer a Failed condition with a non-empty message.
		if failed := apimeta.FindStatusCondition(nimcache.Status.Conditions, "Failed"); failed != nil && failed.Message != "" {
			msgCond = failed
		} else if msgCond.Message == "" {
			// Fallback: find the first condition with a non-empty message.
			for i := range nimcache.Status.Conditions {
				if nimcache.Status.Conditions[i].Message != "" {
					msgCond = &nimcache.Status.Conditions[i]
					break
				}
			}
		}

		resTable.Rows = append(resTable.Rows, v1.TableRow{
			Cells: []interface{}{
				nimcache.GetName(),
				nimcache.GetNamespace(),
				nimcache.Status.State,
				nimcache.Status.PVC,
				fmt.Sprintf("%s/%s", msgCond.Type, msgCond.Status),
				msgCond.LastTransitionTime,
				msgCond.Message,
				age,
			},
		})
	}

	return resultTablePrinter.PrintObj(resTable, output)
}

// printSingleNIMCache prints a human-readable paragraph describing a single NIMCache.
func printSingleNIMCache(nimcache *appsv1alpha1.NIMCache, output io.Writer) error {
	age := duration.HumanDuration(time.Since(nimcache.GetCreationTimestamp().Time))
	if nimcache.GetCreationTimestamp().Time.IsZero() {
		age = "<unknown>"
	}

	msgCond, err := MessageCondition(nimcache)

	paragraph := fmt.Sprintf(
		"Name: %s\nNamespace: %s\nState: %s\nPVC: %s\nType/Status: %s/%s\nLast Transition Time: %s\nMessage: %s\nAge: %s\nCached NIM Profiles:\n",
		nimcache.GetName(),
		nimcache.GetNamespace(),
		nimcache.Status.State,
		nimcache.Status.PVC,
		msgCond.Type,
		msgCond.Status,
		msgCond.LastTransitionTime,
		msgCond.Message,
		age,
	)

	var profileLines string
	for _, p := range nimcache.Status.Profiles {
		profileLines += fmt.Sprintf("  Name: %s, Model: %s, Release: %s, Config: %v\n", p.Name, p.Model, p.Release, p.Config)
	}

	_, err = fmt.Fprint(output, paragraph+profileLines)
	return err
}
