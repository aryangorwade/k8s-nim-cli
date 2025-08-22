package status

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"k8s-nim-operator-cli/pkg/util/client"

	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

type StatusResourceOptions struct {
	cmdFactory    cmdutil.Factory
	ioStreams     *genericclioptions.IOStreams
	namespace     string
	resourcename  string
	allNamespaces bool
}

func NewStatusResourceOptions(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *StatusResourceOptions {
	return &StatusResourceOptions{
		cmdFactory: cmdFactory,
		ioStreams:  &streams,
	}
}

// Populates GetResourceOptions with namespace and resource name (if present).
func (options *StatusResourceOptions) CompleteNamespace(args []string, cmd *cobra.Command) error {
	namespace, err := cmd.Flags().GetString("namespace")
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}
	options.namespace = namespace
	if options.namespace == "" {
		options.namespace = "default"
	}

	if len(args) >= 1 {
		options.resourcename = args[0]
	}

	return nil
}

func statusResources(ctx context.Context, options *StatusResourceOptions, k8sClient client.Client, resourceListType interface{}) (interface{}, error) {
	var resourceList interface{}
	var err error

	switch ltype := resourceListType.(type) {
	case appsv1alpha1.NIMServiceList:
		resourceList = ltype
	case appsv1alpha1.NIMCacheList:
		resourceList = ltype
	default:
		return nil, fmt.Errorf("unsupported resource type %T", ltype)
	}

	listopts := v1.ListOptions{}
	if options.resourcename != "" {
		listopts = v1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", options.resourcename),
		}
	}

	if options.allNamespaces {
		// Determine type to populate list if no namespace specified.
		switch resourceListType.(type) {

		case appsv1alpha1.NIMServiceList:
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMServices("").List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMServices for all namespaces: %w", err)
			}

		case appsv1alpha1.NIMCacheList:
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMCaches("").List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMCaches for all namespaces: %w", err)
			}
		}
	} else {
		// Determine type to populate list if namespace specified.
		switch resourceListType.(type) {

		case appsv1alpha1.NIMServiceList:
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMServices(options.namespace).List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMServices for namespace %s: %w", options.namespace, err)
			}

		case appsv1alpha1.NIMCacheList:
			resourceList, err = k8sClient.NIMClient().AppsV1alpha1().NIMCaches(options.namespace).List(ctx, listopts)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve NIMCaches for namespace %s: %w", options.namespace, err)
			}
		}
	}

	switch resourceListType.(type) {

	case appsv1alpha1.NIMServiceList:
		// Cast resourceList to NIMServiceList.
		nimServiceList, ok := resourceList.(*appsv1alpha1.NIMServiceList)
		if !ok {
			return nil, fmt.Errorf("failed to cast resourceList to NIMServiceList")
		}

		if options.resourcename != "" && len(nimServiceList.Items) == 0 {
			errMsg := fmt.Sprintf("NIMService %s not found", options.resourcename)
			if options.allNamespaces {
				errMsg += " in any namespace"
			} else {
				errMsg += fmt.Sprintf(" in namespace %s", options.namespace)
			}
			return nil, errors.New(errMsg)
		}

	case appsv1alpha1.NIMCacheList:
		// Cast resourceList to NIMCacheList.
		nimCacheList, ok := resourceList.(*appsv1alpha1.NIMCacheList)
		if !ok {
			return nil, fmt.Errorf("failed to cast resourceList to NIMCacheList")
		}

		if options.resourcename != "" && len(nimCacheList.Items) == 0 {
			errMsg := fmt.Sprintf("NIMCache %s not found", options.resourcename)
			if options.allNamespaces {
				errMsg += " in any namespace"
			} else {
				errMsg += fmt.Sprintf(" in namespace %s", options.namespace)
			}
			return nil, errors.New(errMsg)
		}
	}

	return resourceList, nil
}

func (options *StatusResourceOptions) Run(ctx context.Context, k8sClient client.Client, resourceListType interface{}) error {
	resourceList, err := statusResources(ctx, options, k8sClient, resourceListType)
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
		return printNIMServices(nimServiceList, options.ioStreams.Out)

	case appsv1alpha1.NIMCacheList:
		// Cast resourceList to NIMCacheList.
		nimCacheList, ok := resourceList.(*appsv1alpha1.NIMCacheList)
		if !ok {
			return fmt.Errorf("failed to cast resourceList to NIMCacheList")
		}
		// Determine if a single NIMCache was requested and returned
		if options.resourcename != "" && len(nimCacheList.Items) == 1 {
			return printSingleNIMCache(&nimCacheList.Items[0], options.ioStreams.Out)
		}
		return printNIMCaches(nimCacheList, options.ioStreams.Out)
	}

	return err
}

func MessageCondition(nimcache *appsv1alpha1.NIMCache) (*v1.Condition, error) {
	cond := apimeta.FindStatusCondition(nimcache.Status.Conditions, "Ready")
	if cond == nil {
		return nil, fmt.Errorf("the Ready condition is not set yet")
	}

	msgCond := cond
	// Prefer a Failed condition with a non-empty message
	if failed := apimeta.FindStatusCondition(nimcache.Status.Conditions, "Failed"); failed != nil && failed.Message != "" {
		msgCond = failed
	} else if msgCond.Message == "" {
		// Fallback: first condition with a non-empty message
		for i := range nimcache.Status.Conditions {
			if nimcache.Status.Conditions[i].Message != "" {
				msgCond = &nimcache.Status.Conditions[i]
				break
			}
		}
	}

	return msgCond, nil
}


