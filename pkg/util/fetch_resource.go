package util

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Events        bool
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
	// When logs calls this command, ResourceName will immediately be overwritten by a blank string.
	if len(args) == 1 {
		options.ResourceName = args[0]
	}
	// There would be exactly two arguments if delete calls this (nim DELETE NIMSERVICE META-LLAMA-3B).
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

// Streams logs from all pods as they come.
func StreamResourceLogs(ctx context.Context, options *FetchResourceOptions, k8sClient client.Client, namespace string, resourceName string, labelSelector string) error {
	kube := k8sClient.KubernetesClient()
	pods, err := kube.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s/%s (selector=%q)", namespace, resourceName, labelSelector)
	}

	type logLine struct {
		pod, container string
		text           string
	}
	lines := make(chan logLine, 1024)

	// Flags (replace with real flags)
	follow := true
	allContainers := true
	targetContainer := ""
	timestamps := false

	var wg sync.WaitGroup
	for _, pod := range pods.Items {
		containers := pod.Spec.Containers
		if !allContainers && targetContainer != "" {
			containers = []corev1.Container{{Name: targetContainer}}
		}
		for _, c := range containers {
			wg.Add(1)
			podName, containerName := pod.Name, c.Name
			go func() {
				defer wg.Done()
				req := kube.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
					Container:  containerName,
					Follow:     follow,
					Timestamps: timestamps,
				})
				rc, err := req.Stream(ctx)
				if err != nil {
					fmt.Fprintf(options.IoStreams.ErrOut, "error streaming %s/%s[%s]: %v\n", namespace, podName, containerName, err)
					return
				}
				defer rc.Close()

				sc := bufio.NewScanner(rc)
				for sc.Scan() {
					select {
					case <-ctx.Done():
						return
					case lines <- logLine{pod: podName, container: containerName, text: sc.Text()}:
					}
				}
			}()
		}
	}

	// Close channel when all streams end
	go func() {
		wg.Wait()
		close(lines)
	}()

	// Printer: interleaves lines as they arrive
	for ln := range lines {
		fmt.Fprintf(options.IoStreams.Out, "[%s/%s] %s\n", ln.pod, ln.container, ln.text)
	}
	return nil
}

// StreamResourceEvents lists pods by label selector and prints Kubernetes Events related to those pods as they come.
func StreamResourceEvents(ctx context.Context, options *FetchResourceOptions, k8sClient client.Client, namespace string, resourceName string, labelSelector string) error {
	kube := k8sClient.KubernetesClient()
	pods, err := kube.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s/%s (selector=%q)", namespace, resourceName, labelSelector)
	}

	fmt.Fprintf(options.IoStreams.Out, "Watching events for %d pod(s) in %s matching %q...\n", len(pods.Items), namespace, labelSelector)

	type line struct {
		text string
	}
	lines := make(chan line, 1024)

	var wg sync.WaitGroup
	for _, pod := range pods.Items {
		podName := pod.Name
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Prefer events.k8s.io/v1
			ev1fs := fields.SelectorFromSet(fields.Set{
				"regarding.kind": "Pod",
				"regarding.name": podName,
			})

			// Attempt EventsV1 first
			if elist, err := kube.EventsV1().Events(namespace).List(ctx, v1.ListOptions{FieldSelector: ev1fs.String()}); err == nil {
				items := elist.Items
				sort.Slice(items, func(i, j int) bool {
					ti := items[i].EventTime.Time
					if ti.IsZero() && items[i].Series != nil && !items[i].Series.LastObservedTime.Time.IsZero() {
						ti = items[i].Series.LastObservedTime.Time
					}
					tj := items[j].EventTime.Time
					if tj.IsZero() && items[j].Series != nil && !items[j].Series.LastObservedTime.Time.IsZero() {
						tj = items[j].Series.LastObservedTime.Time
					}
					return ti.Before(tj)
				})
				for _, ev := range items {
					ts := ev.EventTime.Time
					if ts.IsZero() && ev.Series != nil && !ev.Series.LastObservedTime.Time.IsZero() {
						ts = ev.Series.LastObservedTime.Time
					}
					tsStr := ts.Format(time.RFC3339)
					src := ev.ReportingController
					if src == "" {
						src = "kubelet"
					}
					select {
					case <-ctx.Done():
						return
					case lines <- line{text: fmt.Sprintf("[%s] %s %s %s %s: %s", podName, tsStr, ev.Type, ev.Reason, src, ev.Note)}:
					}
				}

				w, err := kube.EventsV1().Events(namespace).Watch(ctx, v1.ListOptions{
					FieldSelector:       ev1fs.String(),
					ResourceVersion:     elist.ResourceVersion,
					AllowWatchBookmarks: true,
					Watch:               true,
				})
				if err != nil {
					fmt.Fprintf(options.IoStreams.ErrOut, "error watching v1 events for pod %s/%s: %v\n", namespace, podName, err)
					return
				}
				defer w.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case evt, ok := <-w.ResultChan():
						if !ok {
							return
						}
						ev, ok := evt.Object.(*eventsv1.Event)
						if !ok || ev == nil {
							continue
						}
						ts := ev.EventTime.Time
						if ts.IsZero() && ev.Series != nil && !ev.Series.LastObservedTime.Time.IsZero() {
							ts = ev.Series.LastObservedTime.Time
						}
						tsStr := ts.Format(time.RFC3339)
						src := ev.ReportingController
						if src == "" {
							src = "kubelet"
						}
						select {
						case <-ctx.Done():
							return
						case lines <- line{text: fmt.Sprintf("[%s] %s %s %s %s: %s", podName, tsStr, ev.Type, ev.Reason, src, ev.Note)}:
						}
					}
				}
			}

			// Fallback to core/v1 events if EventsV1 list errored
			fs := fields.SelectorFromSet(fields.Set{
				"involvedObject.kind": "Pod",
				"involvedObject.name": podName,
			})

			elist, err := kube.CoreV1().Events(namespace).List(ctx, v1.ListOptions{FieldSelector: fs.String()})
			if err != nil {
				fmt.Fprintf(options.IoStreams.ErrOut, "error listing core/v1 events for pod %s/%s: %v\n", namespace, podName, err)
				return
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
				select {
				case <-ctx.Done():
					return
				case lines <- line{text: fmt.Sprintf("[%s] %s %s %s %s: %s", podName, tsStr, ev.Type, ev.Reason, src, ev.Message)}:
				}
			}

			w, err := kube.CoreV1().Events(namespace).Watch(ctx, v1.ListOptions{
				FieldSelector:       fs.String(),
				ResourceVersion:     elist.ResourceVersion,
				AllowWatchBookmarks: true,
				Watch:               true,
			})
			if err != nil {
				fmt.Fprintf(options.IoStreams.ErrOut, "error watching core/v1 events for pod %s/%s: %v\n", namespace, podName, err)
				return
			}
			defer w.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-w.ResultChan():
					if !ok {
						return
					}
					ev, ok := evt.Object.(*corev1.Event)
					if !ok || ev == nil {
						continue
					}
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
					select {
					case <-ctx.Done():
						return
					case lines <- line{text: fmt.Sprintf("[%s] %s %s %s %s: %s", podName, tsStr, ev.Type, ev.Reason, src, ev.Message)}:
					}
				}
			}
		}()
	}

	// Close the shared channel when all goroutines complete
	go func() {
		wg.Wait()
		close(lines)
	}()

	// Interleave output from all pods as it arrives
	for ln := range lines {
		fmt.Fprintln(options.IoStreams.Out, ln.text)
	}
	return nil
}
