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

package infrastructure_test

import (
	"fmt"
	testinfra "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/internal/test/infrastructure"
	"github.com/gardener/gardener-extensions/pkg/mock/gardener-extensions/gardener/terraformer"
	"github.com/golang/mock/gomock"

	gcpv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/internal"
	. "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extensions/pkg/controller"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Terraform", func() {
	var (
		ctrl               *gomock.Controller
		vpcName            string
		infra              *extensionsv1alpha1.Infrastructure
		config             *gcpv1alpha1.InfrastructureConfig
		cluster            *controller.Cluster
		projectID          string
		serviceAccountData []byte
		serviceAccount     *internal.ServiceAccount

		serviceAccountEmail string
		subnetNodes         string
		subnetInternal      string
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		internalCIDR := gardencorev1alpha1.CIDR("192.168.0.0/16")

		config = &gcpv1alpha1.InfrastructureConfig{
			Networks: gcpv1alpha1.NetworkConfig{
				VPC: &gcpv1alpha1.VPC{
					Name: vpcName,
				},
				Internal: &internalCIDR,
				Worker:   gardencorev1alpha1.CIDR("10.1.0.0/16"),
			},
		}

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},

			Spec: extensionsv1alpha1.InfrastructureSpec{
				Region: "eu-west-1",
				SecretRef: corev1.SecretReference{
					Namespace: "foo",
					Name:      "gcp-credentials",
				},
				ProviderConfig: &runtime.RawExtension{
					Object: config,
				},
			},
		}

		podsCIDR := gardencorev1alpha1.CIDR("11.0.0.0/16")
		servicesCIDR := gardencorev1alpha1.CIDR("12.0.0.0/16")
		cluster = &controller.Cluster{
			Shoot: &gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						GCP: &gardenv1beta1.GCPCloud{
							Networks: gardenv1beta1.GCPNetworks{
								K8SNetworks: gardencorev1alpha1.K8SNetworks{
									Pods:     &podsCIDR,
									Services: &servicesCIDR,
								},
							},
						},
					},
				},
			},
		}

		projectID = "project"
		serviceAccountData = []byte(fmt.Sprintf(`{"project_id": "%s"}`, projectID))
		serviceAccount = &internal.ServiceAccount{ProjectID: projectID, Raw: serviceAccountData}
		serviceAccountEmail = "gardener@cloud"
		subnetNodes = "nodes-subnet"
		subnetInternal = "internal"
	})

	Describe("#ComputeTerraformerChartValues", func() {
		It("should correctly compute the terraformer chart values", func() {
			values := ComputeTerraformerChartValues(infra, serviceAccount, config, cluster)

			Expect(values).To(Equal(map[string]interface{}{
				"google": map[string]interface{}{
					"region":  infra.Spec.Region,
					"project": projectID,
				},
				"create": map[string]interface{}{
					"vpc": false,
				},
				"vpc": map[string]interface{}{
					"name": config.Networks.VPC.Name,
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"pods":     cluster.Shoot.Spec.Cloud.GCP.Networks.Pods,
					"services": cluster.Shoot.Spec.Cloud.GCP.Networks.Services,
					"worker":   config.Networks.Worker,
					"internal": config.Networks.Internal,
				},
				"outputKeys": map[string]interface{}{
					"vpcName":             TerraformerOutputKeyVPCName,
					"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
					"subnetNodes":         TerraformerOutputKeySubnetNodes,
					"subnetInternal":      TerraformerOutputKeySubnetInternal,
				},
			}))
		})

		It("should correctly compute the terraformer chart values with vpc creation", func() {
			config.Networks.VPC = nil
			values := ComputeTerraformerChartValues(infra, serviceAccount, config, cluster)

			Expect(values).To(Equal(map[string]interface{}{
				"google": map[string]interface{}{
					"region":  infra.Spec.Region,
					"project": projectID,
				},
				"create": map[string]interface{}{
					"vpc": true,
				},
				"vpc": map[string]interface{}{
					"name": DefaultVPCName,
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"pods":     cluster.Shoot.Spec.Cloud.GCP.Networks.Pods,
					"services": cluster.Shoot.Spec.Cloud.GCP.Networks.Services,
					"worker":   config.Networks.Worker,
					"internal": config.Networks.Internal,
				},
				"outputKeys": map[string]interface{}{
					"vpcName":             TerraformerOutputKeyVPCName,
					"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
					"subnetNodes":         TerraformerOutputKeySubnetNodes,
					"subnetInternal":      TerraformerOutputKeySubnetInternal,
				},
			}))
		})
	})

	Describe("#StatusFromTerraformState", func() {
		var (
			state *TerraformState
		)

		BeforeEach(func() {
			state = &TerraformState{
				VPCName:             vpcName,
				ServiceAccountEmail: serviceAccountEmail,
				SubnetNodes:         subnetNodes,
				SubnetInternal:      &subnetInternal,
			}
		})

		It("should correctly compute the status", func() {
			status := StatusFromTerraformState(state)

			Expect(status).To(Equal(&gcpv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				Networks: gcpv1alpha1.NetworkStatus{
					VPC: gcpv1alpha1.VPC{
						Name: vpcName,
					},
					Subnets: []gcpv1alpha1.Subnet{
						{
							Purpose: gcpv1alpha1.PurposeNodes,
							Name:    subnetNodes,
						},
						{
							Purpose: gcpv1alpha1.PurposeInternal,
							Name:    subnetInternal,
						},
					},
				},
				ServiceAccountEmail: serviceAccountEmail,
			}))
		})

		It("should correctly compute the status without internal subnet", func() {
			state.SubnetInternal = nil
			status := StatusFromTerraformState(state)

			Expect(status).To(Equal(&gcpv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				Networks: gcpv1alpha1.NetworkStatus{
					VPC: gcpv1alpha1.VPC{
						Name: vpcName,
					},
					Subnets: []gcpv1alpha1.Subnet{
						{
							Purpose: gcpv1alpha1.PurposeNodes,
							Name:    subnetNodes,
						},
					},
				},
				ServiceAccountEmail: serviceAccountEmail,
			}))
		})
	})

	Describe("#ExtractTerraformState", func() {
		It("should correctly extract the terraform state", func() {
			tf := terraformer.NewMockTerraformer(ctrl)

			tf.EXPECT().GetStateOutputVariables(
				TerraformerOutputKeyVPCName,
				TerraformerOutputKeySubnetNodes,
				TerraformerOutputKeyServiceAccountEmail,
				TerraformerOutputKeySubnetInternal,
			).Return(testinfra.MkTerraformerOutputVariables(vpcName, subnetNodes, serviceAccountEmail, &subnetInternal), nil)

			state, err := ExtractTerraformState(tf, config)
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(&TerraformState{
				VPCName:             vpcName,
				ServiceAccountEmail: serviceAccountEmail,
				SubnetNodes:         subnetNodes,
				SubnetInternal:      &subnetInternal,
			}))
		})
	})
})
