// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"github.com/gardener/gardener-extensions/pkg/gardener/terraformer"
	"path/filepath"

	gcpv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/internal"
	"github.com/gardener/gardener-extensions/pkg/controller"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultVPCName is the default VPC terraform name.
	DefaultVPCName = "${google_compute_network.network.name}"

	// TerraformerPurpose is the terraformer infrastructure purpose.
	TerraformerPurpose = "infra"

	// TerraformerOutputKeyVPCName is the name of the vpc_name terraform output variable.
	TerraformerOutputKeyVPCName = "vpc_name"
	// TerraformerOutputKeyServiceAccountEmail is the name of the service_account_email terraform output variable.
	TerraformerOutputKeyServiceAccountEmail = "service_account_email"
	// TerraformerOutputKeySubnetNodes is the name of the subnet_nodes terraform output variable.
	TerraformerOutputKeySubnetNodes = "subnet_nodes"
	// TerraformerOutputKeySubnetInternal is the name of the subnet_internal terraform output variable.
	TerraformerOutputKeySubnetInternal = "subnet_internal"

	// InfraChartName is the name of the gcp-infra chart.
	InfraChartName = "gcp-infra"
)

var (
	// ChartsPath is the path to the charts
	ChartsPath = filepath.Join("controllers", "provider-gcp", "charts")
	// InternalChartsPath is the path to the internal charts
	InternalChartsPath = filepath.Join(ChartsPath, "internal")

	// InfraChartPath is the path to the gcp-infra chart.
	InfraChartPath = filepath.Join(InternalChartsPath, "gcp-infra")

	// StatusTypeMeta is the TypeMeta of the GCP InfrastructureStatus
	StatusTypeMeta = metav1.TypeMeta{
		APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
		Kind:       "InfrastructureStatus",
	}
)

// getK8SNetworks gets the K8SNetworks from the given controller.Cluster.
func getK8SNetworks(cluster *controller.Cluster) *gardencorev1alpha1.K8SNetworks {
	return &cluster.Shoot.Spec.Cloud.GCP.Networks.K8SNetworks
}

// ComputeTerraformerChartValues computes the values for the GCP Terraformer chart.
func ComputeTerraformerChartValues(
	infra *extensionsv1alpha1.Infrastructure,
	account *internal.ServiceAccount,
	config *gcpv1alpha1.InfrastructureConfig,
	cluster *controller.Cluster,
) map[string]interface{} {
	var (
		vpcName   = DefaultVPCName
		createVPC = true
	)

	networks := getK8SNetworks(cluster)

	if config.Networks.VPC != nil {
		createVPC = false
		vpcName = config.Networks.VPC.Name
	}

	return map[string]interface{}{
		"google": map[string]interface{}{
			"region":  infra.Spec.Region,
			"project": account.ProjectID,
		},
		"create": map[string]interface{}{
			"vpc": createVPC,
		},
		"vpc": map[string]interface{}{
			"name": vpcName,
		},
		"clusterName": infra.Namespace,
		"networks": map[string]interface{}{
			"pods":     networks.Pods,
			"services": networks.Services,
			"worker":   config.Networks.Worker,
			"internal": config.Networks.Internal,
		},
		"outputKeys": map[string]interface{}{
			"vpcName":             TerraformerOutputKeyVPCName,
			"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
			"subnetNodes":         TerraformerOutputKeySubnetNodes,
			"subnetInternal":      TerraformerOutputKeySubnetInternal,
		},
	}
}

// RenderTerraformerChart renders the gcp-infra chart with the given values.
func RenderTerraformerChart(
	renderer chartrenderer.Interface,
	infra *extensionsv1alpha1.Infrastructure,
	account *internal.ServiceAccount,
	config *gcpv1alpha1.InfrastructureConfig,
	cluster *controller.Cluster,
) (*TerraformFiles, error) {
	values := ComputeTerraformerChartValues(infra, account, config, cluster)

	release, err := renderer.Render(InfraChartPath, InfraChartName, infra.Namespace, values)
	if err != nil {
		return nil, err
	}

	return &TerraformFiles{
		Main:      release.FileContent("main.tf"),
		Variables: release.FileContent("variables.tf"),
		TFVars:    []byte(release.FileContent("terraform.tfvars")),
	}, nil
}

// TerraformFiles are the files that have been rendered from the infrastructure chart.
type TerraformFiles struct {
	Main      string
	Variables string
	TFVars    []byte
}

// TerraformState is the Terraform state for an infrastructure.
type TerraformState struct {
	// VPCName is the name of the VPC created for an infrastructure.
	VPCName string
	// ServiceAccountEmail is the service account email for a network.
	ServiceAccountEmail string
	// SubnetNodes is the CIDR of the nodes subnet of an infrastructure.
	SubnetNodes string
	// SubnetInternal is the CIDR of the internal subnet of an infrastructure.
	SubnetInternal *string
}

// ExtractTerraformState extracts the TerraformState from the given Terraformer.
func ExtractTerraformState(tf terraformer.Terraformer, config *gcpv1alpha1.InfrastructureConfig) (*TerraformState, error) {
	outputKeys := []string{
		TerraformerOutputKeyVPCName,
		TerraformerOutputKeySubnetNodes,
		TerraformerOutputKeyServiceAccountEmail,
	}

	hasInternal := config.Networks.Internal != nil
	if hasInternal {
		outputKeys = append(outputKeys, TerraformerOutputKeySubnetInternal)
	}

	vars, err := tf.GetStateOutputVariables(outputKeys...)
	if err != nil {
		return nil, err
	}

	state := &TerraformState{
		VPCName:             vars[TerraformerOutputKeyVPCName],
		SubnetNodes:         vars[TerraformerOutputKeySubnetNodes],
		ServiceAccountEmail: vars[TerraformerOutputKeyServiceAccountEmail],
	}
	if hasInternal {
		subnetInternal := vars[TerraformerOutputKeySubnetInternal]
		state.SubnetInternal = &subnetInternal
	}
	return state, nil
}

// StatusFromTerraformState computes an InfrastructureStatus from the given
// Terraform variables.
func StatusFromTerraformState(state *TerraformState) *gcpv1alpha1.InfrastructureStatus {
	var (
		status = &gcpv1alpha1.InfrastructureStatus{
			TypeMeta: StatusTypeMeta,
			Networks: gcpv1alpha1.NetworkStatus{
				VPC: gcpv1alpha1.VPC{
					Name: state.VPCName,
				},
				Subnets: []gcpv1alpha1.Subnet{
					{
						Purpose: gcpv1alpha1.PurposeNodes,
						Name:    state.SubnetNodes,
					},
				},
			},
			ServiceAccountEmail: state.ServiceAccountEmail,
		}
	)

	if state.SubnetInternal != nil {
		status.Networks.Subnets = append(status.Networks.Subnets, gcpv1alpha1.Subnet{
			Purpose: gcpv1alpha1.PurposeInternal,
			Name:    *state.SubnetInternal,
		})
	}
	return status
}

// ComputeStatus computes the status based on the Terraformer and the given InfrastructureConfig.
func ComputeStatus(tf terraformer.Terraformer, config *gcpv1alpha1.InfrastructureConfig) (*gcpv1alpha1.InfrastructureStatus, error) {
	state, err := ExtractTerraformState(tf, config)
	if err != nil {
		return nil, err
	}

	return StatusFromTerraformState(state), nil
}
