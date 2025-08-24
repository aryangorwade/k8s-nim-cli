package util

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"k8s-nim-operator-cli/pkg/util/client"

	apimeta "k8s.io/apimachinery/pkg/api/meta"

	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

type FetchResourceOptions struct {
	cmdFactory    cmdutil.Factory
	IoStreams     *genericclioptions.IOStreams
	namespace     string
	ResourceName  string
	ResourceType  ResourceType
	AllNamespaces bool
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
	options.namespace = namespace
	if options.namespace == "" {
		options.namespace = "default"
	}

	// When get and status call this, there will only ever be one argument at most (nim get NIMSERVICE NAME or nim get NIMSERVICES).
	if len(args) == 1 {
		options.ResourceName = args[0]
	}
	// There would only be two arguments if log calls this (nim LOG NIMSERVICE META-LLAMA-3B).
	if len(args) == 2 {
		options.ResourceType = ResourceType(strings.ToLower(args[0]))
		options.ResourceName = args[1]
	}

	return nil
}

func FetchResources(ctx context.Context, options *FetchResourceOptions, k8sClient client.Client, resourceListType interface{}) (interface{}, error) {
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
	if options.ResourceName != "" {
		listopts = v1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", options.ResourceName),
		}
	}

	if options.AllNamespaces {
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

		if options.ResourceName != "" && len(nimServiceList.Items) == 0 {
			errMsg := fmt.Sprintf("NIMService %s not found", options.ResourceName)
			if options.AllNamespaces {
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

		if options.ResourceName != "" && len(nimCacheList.Items) == 0 {
			errMsg := fmt.Sprintf("NIMCache %s not found", options.ResourceName)
			if options.AllNamespaces {
				errMsg += " in any namespace"
			} else {
				errMsg += fmt.Sprintf(" in namespace %s", options.namespace)
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
