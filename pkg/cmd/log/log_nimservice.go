package log

import (
	"fmt"
	"io"
	"time"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	corev1 "k8s.io/api/core/v1"

	"k8s-nim-operator-cli/pkg/util/client"
	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

func NewLogNIMServiceCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := NewLogResourceOptions(cmdFactory, streams)

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
			return options.Run(cmd.Context(), k8sClient, appsv1alpha1.NIMServiceList{})
		},
	}
	cmd.Flags().BoolVarP(&options.allNamespaces, "all-namespaces", "A", false, "If present, list the requested NIMService's logs across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
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

func getExpose(nimService *appsv1alpha1.NIMService) string {
	var (
		port string
		name = nimService.Spec.Expose.Service.Name
	)

	// Port is an int32 value, not *int32
	if nimService.Spec.Expose.Service.Port != 0 {
		port = fmt.Sprint(nimService.Spec.Expose.Service.Port)
	}

	switch {
	case port != "" && name != "":
		return fmt.Sprintf("Name: %s, Port: %s", name, port)
	case port != "":
		return fmt.Sprintf("Port: %s", port)
	default:
		return ""
	}
}


func getScale(nimService *appsv1alpha1.NIMService) string {
	if nimService.Spec.Scale.Enabled == nil || !*nimService.Spec.Scale.Enabled {
		return "disabled"
	}

	if nimService.Spec.Scale.HPA.MinReplicas != nil {
		return fmt.Sprintf("min: %d, max: %d",
			*nimService.Spec.Scale.HPA.MinReplicas,
			nimService.Spec.Scale.HPA.MaxReplicas)
	}

	return fmt.Sprintf("max: %d", nimService.Spec.Scale.HPA.MaxReplicas)
}

func getStorage(nimService *appsv1alpha1.NIMService) string {
	// If NIMCache is defined.
	if (nimService.Spec.Storage.NIMCache != appsv1alpha1.NIMCacheVolSpec{}) {
		return fmt.Sprintf("NIMCache: name: %s, profile: %s", nimService.Spec.Storage.NIMCache.Name, nimService.Spec.Storage.NIMCache.Profile)
	}

	// If PVC is defined.
	if (nimService.Spec.Storage.PVC != appsv1alpha1.PersistentVolumeClaim{}) {
		if nimService.Spec.Storage.PVC.Name != "" {
			return fmt.Sprintf("PVC: %s, %s", nimService.Spec.Storage.PVC.Name, nimService.Spec.Storage.PVC.Size)
		}
		return fmt.Sprintf("PVC: %s", nimService.Spec.Storage.PVC.Size)	
	}

	// One of NIMCache, PVC, HostPath must be defined. 
	return fmt.Sprintf("HostPath: %s", *nimService.Spec.Storage.HostPath)
}

func getNIMServiceResources(nimService *appsv1alpha1.NIMService) string {
	result := ""
	if !reflect.DeepEqual(nimService.Spec.Resources, corev1.ResourceRequirements{}) {
		// Pretty print limits.
		if len(nimService.Spec.Resources.Limits) != 0 {
			result += fmt.Sprintf("Limits: %s", resourceListToOneLine(nimService.Spec.Resources.Limits))
		}
		// Pretty print requests.
		if len(nimService.Spec.Resources.Requests) != 0 {
			result += fmt.Sprintf("\nRequests: %s", resourceListToOneLine(nimService.Spec.Resources.Requests))
		}
		// Pretty print claims.
		if len(nimService.Spec.Resources.Claims) != 0 {
			result += fmt.Sprintf("\nClaims: %s", claimsToOneLine(nimService.Spec.Resources.Claims))
		}
	}

	return ""
}

func resourceListToOneLine(rl corev1.ResourceList) string {
    var parts []string

    // Sort keys for stable output
    keys := make([]string, 0, len(rl))
    for k := range rl {
        keys = append(keys, string(k))
    }
    sort.Strings(keys)

    for _, k := range keys {
        v := rl[corev1.ResourceName(k)]
        parts = append(parts, fmt.Sprintf("%s: %s", k, v.String()))
    }

    return strings.Join(parts, ", ")
}

func claimsToOneLine(claims []corev1.ResourceClaim) string {
    var parts []string
    for _, c := range claims {
        parts = append(parts, fmt.Sprintf("%s(%s)", c.Name, c.Request))
    }
    return strings.Join(parts, ", ")
}