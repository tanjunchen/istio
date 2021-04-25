// Copyright 2019 Istio Authors
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

package name

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ghodss/yaml"

	"istio.io/api/operator/v1alpha1"
	iop "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	"istio.io/istio/operator/pkg/helm"
	"istio.io/istio/operator/pkg/tpath"
	"istio.io/istio/operator/pkg/vfs"
	"istio.io/istio/operator/version"
)

// Kubernetes Kind strings.
const (
	CRDStr                   = "CustomResourceDefinition"
	DaemonSetStr             = "DaemonSet"
	DeploymentStr            = "Deployment"
	HPAStr                   = "HorizontalPodAutoscaler"
	NamespaceStr             = "Namespace"
	PodStr                   = "Pod"
	PDBStr                   = "PodDisruptionBudget"
	ReplicationControllerStr = "ReplicationController"
	ReplicaSetStr            = "ReplicaSet"
	RoleStr                  = "Role"
	RoleBindingStr           = "RoleBinding"
	SAStr                    = "ServiceAccount"
	ServiceStr               = "Service"
	StatefulSetStr           = "StatefulSet"
)

const (
	// OperatorAPINamespace is the API namespace for operator config.
	// TODO: move this to a base definitions file when one is created.
	OperatorAPINamespace = "operator.istio.io"
	// ConfigFolder is the folder where we store translation configurations
	ConfigFolder = "translateConfig"
	// ConfigPrefix is the prefix of IstioOperator's translation configuration file
	ConfigPrefix = "names-"
	// DefaultProfileName is the name of the default profile.
	DefaultProfileName = "default"
)

// ComponentName is a component name string, typed to constrain allowed values.
type ComponentName string

const (
	// IstioComponent names corresponding to the IstioOperator proto component names. Must be the same, since these
	// are used for struct traversal.
	IstioBaseComponentName ComponentName = "Base"
	PilotComponentName     ComponentName = "Pilot"
	PolicyComponentName    ComponentName = "Policy"
	TelemetryComponentName ComponentName = "Telemetry"

	CNIComponentName ComponentName = "Cni"

	// istiod remote component
	IstiodRemoteComponentName ComponentName = "IstiodRemote"

	// Gateway components
	IngressComponentName ComponentName = "IngressGateways"
	EgressComponentName  ComponentName = "EgressGateways"

	// Addon root component
	AddonComponentName ComponentName = "AddonComponents"

	// Operator components
	IstioOperatorComponentName      ComponentName = "IstioOperator"
	IstioOperatorCustomResourceName ComponentName = "IstioOperatorCustomResource"

	IstioDefaultNamespace = "istio-system"
)

// ComponentNamesConfig is used for unmarshaling legacy and addon naming data.
type ComponentNamesConfig struct {
	DeprecatedComponentNames []string
}

var (
	AllCoreComponentNames = []ComponentName{
		IstioBaseComponentName,
		PilotComponentName,
		PolicyComponentName,
		TelemetryComponentName,
		CNIComponentName,
		IstiodRemoteComponentName,
	}

	allComponentNamesMap = make(map[ComponentName]bool)
	// DeprecatedComponentNamesMap defines the names of deprecated istio core components used in old versions,
	// which would not appear as standalone components in current version. This is used for pruning, and alerting
	// users to the fact that the components are deprecated.
	DeprecatedComponentNamesMap = make(map[ComponentName]bool)

	// BundledAddonComponentNamesMap is a map of component names of addons which have helm charts bundled with Istio
	// and have built in path definitions beyond standard addons coming from external charts.
	BundledAddonComponentNamesMap = make(map[ComponentName]bool)

	// ValuesEnablementPathMap defines a mapping between legacy values enablement paths and the corresponding enablement
	// paths in IstioOperator.
	ValuesEnablementPathMap = map[string]string{
		"spec.values.gateways.istio-ingressgateway.enabled": "spec.components.ingressGateways.[name:istio-ingressgateway].enabled",
		"spec.values.gateways.istio-egressgateway.enabled":  "spec.components.egressGateways.[name:istio-egressgateway].enabled",
	}

	// userFacingComponentNames are the names of components that are displayed to the user in high level CLIs
	// (like progress log).
	userFacingComponentNames = map[ComponentName]string{
		IstioBaseComponentName:          "Istio core",
		PilotComponentName:              "Istiod",
		PolicyComponentName:             "Policy",
		TelemetryComponentName:          "Telemetry",
		CNIComponentName:                "CNI",
		IngressComponentName:            "Ingress gateways",
		EgressComponentName:             "Egress gateways",
		AddonComponentName:              "Addons",
		IstioOperatorComponentName:      "Istio operator",
		IstioOperatorCustomResourceName: "Istio operator CRDs",
		IstiodRemoteComponentName:       "Istiod remote",
	}
	scanAddons sync.Once
)

// Kubernetes Kind strings.
const (
	ClusterRoleStr                    = "ClusterRole"
	ClusterRoleBindingStr             = "ClusterRoleBinding"
	CMStr                             = "ConfigMap"
	MutatingWebhookConfigurationStr   = "MutatingWebhookConfiguration"
	PVCStr                            = "PersistentVolumeClaim"
	SecretStr                         = "Secret"
	ValidatingWebhookConfigurationStr = "ValidatingWebhookConfiguration"
)

// Istio Kind strings
const (
	EnvoyFilterStr        = "EnvoyFilter"
	GatewayStr            = "Gateway"
	DestinationRuleStr    = "DestinationRule"
	PeerAuthenticationStr = "PeerAuthentication"
	VirtualServiceStr     = "VirtualService"
)

// Istio API Group Names
const (
	NetworkingAPIGroupName = "networking.istio.io"
	SecurityAPIGroupName   = "security.istio.io"
)

var (
	// AllComponentNames is a list of all Istio components.
	AllComponentNames = append(AllCoreComponentNames, IngressComponentName, EgressComponentName,
		IstioOperatorComponentName, IstioOperatorCustomResourceName)

	allCoreComponentNamesMap = map[ComponentName]bool{}
)

func init() {
	for _, n := range AllCoreComponentNames {
		allComponentNamesMap[n] = true
	}
	if err := loadComponentNamesConfig(); err != nil {
		panic(err)
	}
}

// Manifest defines a manifest for a component.
type Manifest struct {
	Name    ComponentName
	Content string
}

// ManifestMap is a map of ComponentName to its manifest string.
type ManifestMap map[ComponentName][]string

// Consolidated returns a representation of mm where all manifests in the slice under a key are combined into a single
// manifest.
func (mm ManifestMap) Consolidated() map[string]string {
	out := make(map[string]string)
	for cname, ms := range mm {
		allM := ""
		for _, m := range ms {
			allM += m + helm.YAMLSeparator
		}
		out[string(cname)] = allM
	}
	return out
}

// MergeManifestSlices merges a slice of manifests into a single manifest string.
func MergeManifestSlices(manifests []string) string {
	return strings.Join(manifests, helm.YAMLSeparator)
}

// String implements the Stringer interface.
func (mm ManifestMap) String() string {
	out := ""
	for _, ms := range mm {
		for _, m := range ms {
			out += m + helm.YAMLSeparator
		}
	}
	return out
}

// IsCoreComponent reports whether cn is a core component.
func (cn ComponentName) IsCoreComponent() bool {
	return allComponentNamesMap[cn]
}

// IsGateway reports whether cn is a gateway component.
func (cn ComponentName) IsGateway() bool {
	return cn == IngressComponentName || cn == EgressComponentName
}

// IsAddon reports whether cn is an addon component.
func (cn ComponentName) IsAddon() bool {
	return cn == AddonComponentName
}

// Namespace returns the namespace for the component. It follows these rules:
// 1. If DefaultNamespace is unset, log and error and return the empty string.
// 2. If the feature and component namespaces are unset, return DefaultNamespace.
// 3. If the feature namespace is set but component name is unset, return the feature namespace.
// 4. Otherwise return the component namespace.
// Namespace assumes that controlPlaneSpec has been validated.
// TODO: remove extra validations when comfort level is high enough.
func Namespace(componentName ComponentName, controlPlaneSpec *v1alpha1.IstioOperatorSpec) (string, error) {

	defaultNamespace := iop.Namespace(controlPlaneSpec)

	componentNodeI, found, err := tpath.GetFromStructPath(controlPlaneSpec, "Components."+string(componentName)+".Namespace")
	if err != nil {
		return "", fmt.Errorf("error in Namepsace GetFromStructPath componentNamespace for component=%s: %s", componentName, err)
	}
	if !found {
		return defaultNamespace, nil
	}
	if componentNodeI == nil {
		return defaultNamespace, nil
	}
	componentNamespace, ok := componentNodeI.(string)
	if !ok {
		return "", fmt.Errorf("component %s enabled has bad type %T, expect string", componentName, componentNodeI)
	}
	if componentNamespace == "" {
		return defaultNamespace, nil
	}
	return componentNamespace, nil
}

// TitleCase returns a capitalized version of n.
func TitleCase(n ComponentName) ComponentName {
	s := string(n)
	return ComponentName(strings.ToUpper(s[0:1]) + s[1:])
}

// loadComponentNamesConfig loads a config that defines version specific components names, such as legacy components
// names that may not otherwise exist in the code.
func loadComponentNamesConfig() error {
	minorVersion := version.OperatorBinaryVersion.MinorVersion
	f := filepath.Join(ConfigFolder, ConfigPrefix+minorVersion.String()+".yaml")
	b, err := vfs.ReadFile(f)
	if err != nil {
		return fmt.Errorf("failed to read naming file: %v", err)
	}
	namesConfig := &ComponentNamesConfig{}
	err = yaml.Unmarshal(b, &namesConfig)
	if err != nil {
		return fmt.Errorf("failed to unmarshal naming config file: %v", err)
	}
	for _, n := range namesConfig.DeprecatedComponentNames {
		DeprecatedComponentNamesMap[ComponentName(n)] = true
	}
	return nil
}

// onceErr is used to report any error returned through once. It must be globally scoped.
var onceErr error

// ScanBundledAddonComponents scans the specified directory for addons distributed with Istio and dynamically creates
// a map that can be used to refer to these component names through an API with dynamic values.
func ScanBundledAddonComponents(chartsRootDir string) error {
	scanAddons.Do(func() {
		if chartsRootDir == "" {
			if onceErr = helm.CheckCompiledInCharts(); onceErr != nil {
				return
			}
		}

		var addonComponentNames []string
		addonComponentNames, onceErr = helm.GetAddonNamesFromCharts(chartsRootDir, true)
		if onceErr != nil {
			onceErr = fmt.Errorf("failed to scan bundled addon components: %v", onceErr)
			return
		}
		for _, an := range addonComponentNames {
			BundledAddonComponentNamesMap[ComponentName(an)] = true
			enablementName := strings.ToLower(an[:1]) + an[1:]
			valuePath := fmt.Sprintf("spec.values.%s.enabled", enablementName)
			iopPath := fmt.Sprintf("spec.addonComponents.%s.enabled", enablementName)
			ValuesEnablementPathMap[valuePath] = iopPath
		}
	})
	return onceErr
}

// UserFacingComponentName returns the name of the given component that should be displayed to the user in high
// level CLIs (like progress log).
func UserFacingComponentName(name ComponentName) string {
	ret, ok := userFacingComponentNames[name]
	if !ok {
		return "Unknown"
	}
	return ret
}
