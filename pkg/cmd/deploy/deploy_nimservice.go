package deploy

import (
	"fmt"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1 "k8s.io/api/core/v1"

	"context"
	util "k8s-nim-operator-cli/pkg/util"
	"k8s-nim-operator-cli/pkg/util/client"
	"k8s.io/utils/ptr"

	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
)

type NIMServiceOptions struct {
	cmdFactory      cmdutil.Factory
	IoStreams       *genericclioptions.IOStreams
	Namespace       string
	ResourceName    string
	ResourceType    util.ResourceType
	AllNamespaces   bool
	ImageRepository string
	Tag             string
	NIMCacheStorage string
	PVCStorage      string
}

func NewNIMServiceOptions(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *NIMServiceOptions {
	return &NIMServiceOptions{
		cmdFactory: cmdFactory,
		IoStreams:  &streams,
	}
}

// Populates FetchResourceOptions with namespace and resource name (if present).
func (options *NIMServiceOptions) CompleteNamespace(args []string, cmd *cobra.Command) error {
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

	return nil
}

func NewDeployNIMServiceCommand(cmdFactory cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := NewNIMServiceOptions(cmdFactory, streams)

	cmd := &cobra.Command{
		Use:          "nimservice [NAME]",
		Short:        "Deploy new NIMService with specified information.",
		SilenceUsage: true,
		// ValidArgsFunction: completion.RayClusterCompletionFunc(cmdFactory),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cmd.HelpFunc()(cmd, args)
				return nil
			} else {
				// Validate flags first.
				if err := options.ValidateNIMServiceFlags(cmd); err != nil {
					return err
				}

				if err := options.CompleteNamespace(args, cmd); err != nil {
					return err
				}
				// running cmd.Execute or cmd.ExecuteE sets the context, which will be done by root.
				k8sClient, err := client.NewClient(cmdFactory)
				if err != nil {
					return fmt.Errorf("failed to create client: %w", err)
				}
				options.ResourceType = util.NIMService
				return RunDeployNIMService(cmd.Context(), options, k8sClient)
			}
		},
	}

	// The first argument will be name. Other arguments will be specified as flags.
	cmd.Flags().BoolVarP(&options.AllNamespaces, "all-namespaces", "A", false, "If present, list the requested NIMServices across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVar(&options.ImageRepository, "image-repository", util.ImageRepository, "Repository to pull image from.")
	cmd.Flags().StringVar(&options.Tag, "tag", util.Tag, "Image tag.")
	cmd.Flags().StringVar(&options.NIMCacheStorage, "nimcache-storage", util.NIMCacheStorage, "Nimcache name to use for storage.")
	cmd.Flags().StringVar(&options.PVCStorage, "pvc-storage", util.Tag, "PVC name to use for storage.")
	// 	cmd.Flags().StringVar(&options.PVCStorage, "pvc-storage-name", util.Tag, "The PVC name to use for storage.") <= assume create is true

	return cmd
}

// Will need different Run commands for NewDeployNIMCacheCommand and nimservice command.
func RunDeployNIMService(ctx context.Context, options *NIMServiceOptions, k8sClient client.Client) error {

	// Fill out NIMService Spec.
	nimservice := FillOutNIMServiceSpec(options)

	// Set metadata.
	nimservice.Name = options.ResourceName
	nimservice.Namespace = options.Namespace

	// Create the NIMService CR.
	if _, err := k8sClient.NIMClient().AppsV1alpha1().NIMServices(options.Namespace).Create(ctx, nimservice, v1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create NIMService %s/%s: %w", options.Namespace, options.ResourceName, err)
	}

	fmt.Fprintf(options.IoStreams.Out, "NIMService %q created in namespace %q\n", options.ResourceName, options.Namespace)
	return nil
}

func FillOutNIMServiceSpec(options *NIMServiceOptions) *appsv1alpha1.NIMService {
	// Create a sample NIMService.
	nimservice := appsv1alpha1.NIMService{}

	// Fill out NIMService.Spec.
	// Complete Image.
	nimservice.Spec.Image.Repository = options.ImageRepository
	nimservice.Spec.Image.Tag = options.Tag

	// Complete Storage.
	if options.NIMCacheStorage == "" {
		// Use PVC.
		nimservice.Spec.Storage.PVC.Name = options.PVCStorage
		nimservice.Spec.Storage.PVC.Create = ptr.To(false)
	} else {
		// Use NIMCacheStorage.
		nimservice.Spec.Storage.NIMCache.Name = options.NIMCacheStorage
	}
	
	// Lastly, configure default values.
	ConfigureDefaultNIMServiceSpec(options, &nimservice)

	fmt.Printf("DEBUG resources: %+v\n", nimservice.Spec.Resources)

	return &nimservice
}

func ConfigureDefaultNIMServiceSpec(options *NIMServiceOptions, nimservice *appsv1alpha1.NIMService) {
	nimservice.Spec.AuthSecret = util.AuthSecret
	nimservice.Spec.Image.PullSecrets = []string{util.AuthSecret}
	nimservice.Spec.Image.PullPolicy = util.PullPolicy
	nimservice.Spec.Expose.Service.Port = util.ServicePort
	nimservice.Spec.Expose.Service.Type = util.ServiceType
 	
	requirements := &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceName("nvidia.com/gpu"): resource.MustParse(util.GPULimit),
		},
	}
	nimservice.Spec.Resources = requirements
}

func (options *NIMServiceOptions) ValidateNIMServiceFlags(cmd *cobra.Command) error {
	// Required flag field validation.
	// No storage option defined.
	if options.NIMCacheStorage == "" && options.PVCStorage == "" {
		return fmt.Errorf("NIMService's storage source must be provided")
	}
	// No tag defined.
	if options.Tag == "" {
		return fmt.Errorf("NIMService image's tag must be provided")
	}
	// No image repo given.
	if options.ImageRepository == "" {
		return fmt.Errorf("NIMService image repository must be provided")
	}

	return nil
}
