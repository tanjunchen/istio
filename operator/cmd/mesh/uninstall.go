// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mesh

import (
	"fmt"

	"github.com/spf13/cobra"
	"istio.io/api/operator/v1alpha1"
	iopv1alpha1 "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	"istio.io/istio/operator/pkg/cache"
	"istio.io/istio/operator/pkg/helmreconciler"
	"istio.io/istio/operator/pkg/translate"
	"istio.io/istio/operator/pkg/util/clog"
	"istio.io/istio/operator/pkg/util/progress"
	"istio.io/pkg/log"
)

type uninstallArgs struct {
	// kubeConfigPath is the path to kube config file.
	kubeConfigPath string
	// context is the cluster context in the kube config.
	context string
	// skipConfirmation determines whether the user is prompted for confirmation.
	// If set to true, the user is not prompted and a Yes response is assumed in all cases.
	skipConfirmation bool
	// force proceeds even if there are validation errors
	force bool
	// purge results in deletion of all Istio resources.
	purge bool
	// revision is the Istio control plane revision the command targets.
	revision string
	// istioNamespace is the target namespace of istio control plane.
	istioNamespace string
	// filename is the path of input IstioOperator CR.
	filename string
	// set is a string with element format "path=value" where path is an IstioOperator path and the value is a
	// value to set the node at that path to.
	set []string
	// manifestsPath is a path to a charts and profiles directory in the local filesystem, or URL with a release tgz.
	manifestsPath string
}

func addUninstallFlags(cmd *cobra.Command, args *uninstallArgs) {
	cmd.PersistentFlags().StringVarP(&args.kubeConfigPath, "kubeconfig", "c", "", "Path to kube config.")
	cmd.PersistentFlags().StringVar(&args.context, "context", "", "The name of the kubeconfig context to use.")
	cmd.PersistentFlags().BoolVarP(&args.skipConfirmation, "skip-confirmation", "y", false, skipConfirmationFlagHelpStr)
	cmd.PersistentFlags().BoolVar(&args.force, "force", false, "Proceed even with validation errors.")
	cmd.PersistentFlags().BoolVar(&args.purge, "purge", false, "Delete all Istio related sources for all versions")
	cmd.PersistentFlags().StringVarP(&args.revision, "revision", "r", "", revisionFlagHelpStr)
	cmd.PersistentFlags().StringVar(&args.istioNamespace, "istioNamespace", istioDefaultNamespace,
		"The namespace of Istio Control Plane.")
	cmd.PersistentFlags().StringVarP(&args.filename, "filename", "f", "",
		"The filename of the IstioOperator CR.")
	cmd.PersistentFlags().StringVarP(&args.manifestsPath, "manifests", "d", "", chartsFlagHelpStr)
	cmd.PersistentFlags().StringArrayVarP(&args.set, "set", "s", nil, setFlagHelpStr)
}

func UninstallCmd(logOpts *log.Options) *cobra.Command {
	rootArgs := &rootArgs{}
	uiArgs := &uninstallArgs{}
	uicmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Istio from a cluster",
		Long:  "The uninstall command uninstalls Istio from a cluster",
		Example: `  # Uninstall a single control plane by revision
  istioctl x uninstall --revision foo

  # Uninstall a single control plane by iop file
  istioctl x uninstall -f iop.yaml
  
  # Uninstall all control planes and shared resources
  istioctl x uninstall --purge
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if uiArgs.revision == "" && uiArgs.filename == "" && !uiArgs.purge {
				return fmt.Errorf("at least one of the --revision, --filename or --purge flags must be set")
			}
			if len(args) > 0 {
				return fmt.Errorf("istioctl uninstall does not take arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return uninstall(cmd, rootArgs, uiArgs, logOpts)
		}}
	addUninstallFlags(uicmd, uiArgs)
	return uicmd
}

// uninstall uninstalls control plane by either pruning by target revision or deleting specified manifests.
func uninstall(cmd *cobra.Command, rootArgs *rootArgs, uiArgs *uninstallArgs, logOpts *log.Options) error {
	l := clog.NewConsoleLogger(cmd.OutOrStdout(), cmd.ErrOrStderr(), installerScope)
	if err := configLogs(logOpts); err != nil {
		return fmt.Errorf("could not configure logs: %s", err)
	}
	restConfig, _, client, err := K8sConfig(uiArgs.kubeConfigPath, uiArgs.context)
	if err != nil {
		return err
	}
	cache.FlushObjectCaches()
	opts := &helmreconciler.Options{DryRun: rootArgs.dryRun, Log: l, ProgressLog: progress.NewLog()}
	var h *helmreconciler.HelmReconciler

	// If only revision flag is set, we would prune resources by the revision label.
	// Otherwise we would merge the revision flag and the filename flag and delete resources by generated manifests.
	if uiArgs.filename == "" {
		emptyiops := &v1alpha1.IstioOperatorSpec{Profile: "empty", Revision: uiArgs.revision}
		iop, err := translate.IOPStoIOP(emptyiops, "empty", iopv1alpha1.Namespace(emptyiops))
		if err != nil {
			return err
		}
		h, err := helmreconciler.NewHelmReconciler(client, restConfig, iop, opts)
		if err != nil {
			return fmt.Errorf("failed to create reconciler: %v", err)
		}
		objectsList, _, err := h.GetPrunedResources(uiArgs.revision, uiArgs.purge, "")
		if err := h.DeleteObjectsList(objectsList); err != nil {
			return fmt.Errorf("failed to delete control plane resources by revision: %v", err)
		}
		opts.ProgressLog.SetState(progress.StateUninstallComplete)
		return nil
	}
	manifestMap, iops, err := GenManifests([]string{uiArgs.filename},
		applyFlagAliases(uiArgs.set, uiArgs.manifestsPath, uiArgs.revision), uiArgs.force, restConfig, l)
	if err != nil {
		return err
	}
	iop, err := translate.IOPStoIOP(iops, "removed", iopv1alpha1.Namespace(iops))
	if err != nil {
		return err
	}
	h, err = helmreconciler.NewHelmReconciler(client, restConfig, iop, opts)
	if err != nil {
		return fmt.Errorf("failed to create reconciler: %v", err)
	}
	if err := h.DeleteControlPlaneByManifests(manifestMap, iops.Revision, uiArgs.purge); err != nil {
		return fmt.Errorf("failed to delete control plane by manifests: %v", err)
	}
	opts.ProgressLog.SetState(progress.StateUninstallComplete)
	return nil
}
