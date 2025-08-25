package util

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"bufio"
	"sort"
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"k8s-nim-operator-cli/pkg/util/client"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"

	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

type FetchResourceOptions struct {
	cmdFactory    cmdutil.Factory
	IoStreams     *genericclioptions.IOStreams
	Namespace     string
	ResourceName  string
	ResourceType  ResourceType
	AllNamespaces bool
	Events 		  bool
}

func NewFetchResourceOptions(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *FetchResourceOptions {
	return &FetchResourceOptions{
		cmdFactory: cmdFactory,
		IoStreams:  &streams,
	}
}

// Populates FetchResourceOptions with namespace and resource name (if present).
func (options *FetchResourceOptions) CompleteNamespace(args []string, cmd *cobra.Command) error {
	namespace, err := cmd.Flags().GetString("namespace")
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}
	options.Namespace = namespace
	if options.Namespace == "" {
		options.Namespace = "default"
	}

	// When get and status call this, there will only ever be one argument at most (nim get NIMSERVICE NAME or nim get NIMSERVICES).
	if len(args) == 1 {
		options.ResourceName = args[0]
	}
	// There would be exactly two arguments if log calls this (nim LOG NIMSERVICE META-LLAMA-3B).
	if len(args) == 2 {	
		resourceType := ResourceType(strings.ToLower(args[0]))

		// Validating ResourceType.
		switch resourceType {
		case NIMService, NIMCache:
			options.ResourceType = resourceType
		default:
			return fmt.Errorf("invalid resource type %q. Valid types are: nimservice, nimcache", args[0])
		}	

		options.ResourceName = args[1]
	}

	return nil
}

// Returns list of matching resources.
func FetchResources(ctx context.Context, options *FetchResourceOptions, k8sClient client.Client) (interface{}, error) {
	var resourceList interface{}
	var err error


	listopts := v1.ListOptions{}
	if options.ResourceName != "" {
		listopts = v1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", options.ResourceName),
		}
	}

	switch options.ResourceType {

	case NIMService:
		resourceList = appsv1alpha1.NIMServiceList{}

		// Retrieve NIMServices.
		if options.AllNamespaces {
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMServices("").List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMServices for all namespaces: %w", err)
			}
		} else {
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMServices(options.Namespace).List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMServices for namespace %s: %w", options.Namespace, err)
			}
		}

		// Cast resourceList to NIMServiceList.
		nimServiceList, ok := resourceList.(*appsv1alpha1.NIMServiceList)
		if !ok {
			return nil, fmt.Errorf("failed to cast resourceList to NIMServiceList")
		}

		if options.ResourceName != "" && len(nimServiceList.Items) == 0 {
			errMsg := fmt.Sprintf("NIMService %s not found", options.ResourceName)
			if options.AllNamespaces {
				errMsg += " in any namespace"
			} else {
				errMsg += fmt.Sprintf(" in namespace %s", options.Namespace)
			}
			return nil, errors.New(errMsg)
		}
		
	case NIMCache:
		resourceList = appsv1alpha1.NIMCacheList{}

		// Retrieve NIMCaches.
		if options.AllNamespaces {
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMCaches("").List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMCaches for all namespaces: %w", err)
			}
		} else {
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMCaches(options.Namespace).List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMCaches for namespace %s: %w", options.Namespace, err)
			}
		}

		nimCacheList, ok := resourceList.(*appsv1alpha1.NIMCacheList)
		if !ok {
			return nil, fmt.Errorf("failed to cast resourceList to NIMCacheList")
		}

		if options.ResourceName != "" && len(nimCacheList.Items) == 0 {
			errMsg := fmt.Sprintf("NIMCache %s not found", options.ResourceName)
			if options.AllNamespaces {
				errMsg += " in any namespace"
			} else {
				errMsg += fmt.Sprintf(" in namespace %s", options.Namespace)
			}
			return nil, errors.New(errMsg)
		}

	}

	return resourceList, nil
}

func messageConditionFrom(conds []v1.Condition) (*v1.Condition, error) {
	// Prefer a Failed with a non-empty message
	if failed := apimeta.FindStatusCondition(conds, "Failed"); failed != nil && failed.Message != "" {
		return failed, nil
	}
	// Fallback to Ready if present (message may be empty)
	if ready := apimeta.FindStatusCondition(conds, "Ready"); ready != nil {
		return ready, nil
	}
	// Otherwise: first condition with a non-empty message
	for i := range conds {
		if conds[i].Message != "" {
			return &conds[i], nil
		}
	}
	if len(conds) > 0 {
		// Last resort: return the first condition even if it has no message
		return &conds[0], nil
	}
	return nil, fmt.Errorf("no conditions present")
}

func MessageCondition(obj interface{}) (*v1.Condition, error) {
	switch t := obj.(type) {
	case *appsv1alpha1.NIMCache:
		return messageConditionFrom(t.Status.Conditions)
	case *appsv1alpha1.NIMService:
		return messageConditionFrom(t.Status.Conditions)
	default:
		return nil, fmt.Errorf("unsupported type %T (want *NIMCache or *NIMService)", obj)
	}
}

// streamResourceLogs lists pods by label selector in the given namespace and streams their logs.
func StreamResourceLogs(ctx context.Context, options *FetchResourceOptions, k8sClient client.Client, namespace string, resourceName string, labelSelector string) error {
	// List pods using the selector.
	kube := k8sClient.KubernetesClient()
	pods, err := kube.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s/%s (selector=%q)", namespace, resourceName, labelSelector)
	}

	// Stream the pod logs.
	// Replace these with flags if desired:
	follow := true        // e.g., from --follow
	allContainers := true // e.g., from --all-containers
	container := ""       // e.g., from --container

	for _, pod := range pods.Items {
		containers := pod.Spec.Containers
		if !allContainers && container != "" {
			containers = []corev1.Container{{Name: container}}
		}

		for _, c := range containers {
			req := kube.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container:  c.Name,
				Follow:     follow,
				Timestamps: true,
			})
			rc, err := req.Stream(ctx)
			if err != nil {
				// Continue on next container/pod to be robust.
				fmt.Fprintf(options.IoStreams.ErrOut, "error streaming %s/%s[%s]: %v\n", namespace, pod.Name, c.Name, err)
				continue
			}
			// Optional: prefix lines with pod/container
			prefix := fmt.Sprintf("[%s/%s] ", pod.Name, c.Name)
			sc := bufio.NewScanner(rc)
			for sc.Scan() {
				fmt.Fprintln(options.IoStreams.Out, prefix+sc.Text())
			}
			rc.Close()
		}
	}
	return nil
}

// StreamResourceEvents lists pods by label selector and prints Kubernetes Events related to those pods.
func StreamResourceEvents(ctx context.Context, options *FetchResourceOptions, k8sClient client.Client, namespace string, resourceName string, labelSelector string) error {
	kube := k8sClient.KubernetesClient()
	pods, err := kube.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s/%s (selector=%q)", namespace, resourceName, labelSelector)
	}

	for _, pod := range pods.Items {
		fs := fields.SelectorFromSet(fields.Set{
			"involvedObject.kind": "Pod",
			"involvedObject.name": pod.Name,
		})
		elist, err := kube.CoreV1().Events(namespace).List(ctx, v1.ListOptions{FieldSelector: fs.String()})
		if err != nil {
			fmt.Fprintf(options.IoStreams.ErrOut, "error listing events for pod %s/%s: %v\n", namespace, pod.Name, err)
			continue
		}

		items := elist.Items
		sort.Slice(items, func(i, j int) bool {
			ti := items[i].EventTime.Time
			if ti.IsZero() {
				if !items[i].LastTimestamp.Time.IsZero() {
					ti = items[i].LastTimestamp.Time
				} else {
					ti = items[i].FirstTimestamp.Time
				}
			}
			tj := items[j].EventTime.Time
			if tj.IsZero() {
				if !items[j].LastTimestamp.Time.IsZero() {
					tj = items[j].LastTimestamp.Time
				} else {
					tj = items[j].FirstTimestamp.Time
				}
			}
			return ti.Before(tj)
		})

		prefix := fmt.Sprintf("[%s] ", pod.Name)
		for _, ev := range items {
			ts := ev.EventTime.Time
			if ts.IsZero() {
				if !ev.LastTimestamp.Time.IsZero() {
					ts = ev.LastTimestamp.Time
				} else {
					ts = ev.FirstTimestamp.Time
				}
			}
			tsStr := ts.Format(time.RFC3339)
			src := ev.Source.Component
			if src == "" {
				src = "kubelet"
			}
			fmt.Fprintf(options.IoStreams.Out, "%s%s %s %s %s: %s\n", prefix, tsStr, ev.Type, ev.Reason, src, ev.Message)
		}
	}

	return nil
}