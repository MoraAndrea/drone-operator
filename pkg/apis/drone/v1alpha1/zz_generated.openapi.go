// +build !ignore_autogenerated

// This file was autogenerated by openapi-gen. Do not edit it manually!

package v1alpha1

import (
	spec "github.com/go-openapi/spec"
	common "k8s.io/kube-openapi/pkg/common"
)

func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return map[string]common.OpenAPIDefinition{
		"drone-operator/drone-operator/pkg/apis/drone/v1alpha1.DroneFederatedDeployment":       schema_pkg_apis_drone_v1alpha1_DroneFederatedDeployment(ref),
		"drone-operator/drone-operator/pkg/apis/drone/v1alpha1.DroneFederatedDeploymentSpec":   schema_pkg_apis_drone_v1alpha1_DroneFederatedDeploymentSpec(ref),
		"drone-operator/drone-operator/pkg/apis/drone/v1alpha1.DroneFederatedDeploymentStatus": schema_pkg_apis_drone_v1alpha1_DroneFederatedDeploymentStatus(ref),
	}
}

func schema_pkg_apis_drone_v1alpha1_DroneFederatedDeployment(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "DroneFederatedDeployment is the Schema for the dronefederateddeployments API",
				Properties: map[string]spec.Schema{
					"kind": {
						SchemaProps: spec.SchemaProps{
							Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("drone-operator/drone-operator/pkg/apis/drone/v1alpha1.DroneFederatedDeploymentSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("drone-operator/drone-operator/pkg/apis/drone/v1alpha1.DroneFederatedDeploymentStatus"),
						},
					},
				},
			},
		},
		Dependencies: []string{
			"drone-operator/drone-operator/pkg/apis/drone/v1alpha1.DroneFederatedDeploymentSpec", "drone-operator/drone-operator/pkg/apis/drone/v1alpha1.DroneFederatedDeploymentStatus", "k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"},
	}
}

func schema_pkg_apis_drone_v1alpha1_DroneFederatedDeploymentSpec(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "DroneFederatedDeploymentSpec defines the desired state of DroneFederatedDeployment",
				Properties:  map[string]spec.Schema{},
			},
		},
		Dependencies: []string{},
	}
}

func schema_pkg_apis_drone_v1alpha1_DroneFederatedDeploymentStatus(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "DroneFederatedDeploymentStatus defines the observed state of DroneFederatedDeployment",
				Properties:  map[string]spec.Schema{},
			},
		},
		Dependencies: []string{},
	}
}
