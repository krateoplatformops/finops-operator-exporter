/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +kubebuilder:object:generate=true
package v1

import (
	finopsdatatypes "github.com/krateoplatformops/finops-data-types/api/v1"
	prv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ExporterScraperConfig is the Schema for the exporterscraperconfigs API
type ExporterScraperConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   finopsdatatypes.ExporterScraperConfigSpec   `json:"spec,omitempty"`
	Status finopsdatatypes.ExporterScraperConfigStatus `json:"status,omitempty"`
}

func (mg *ExporterScraperConfig) GetCondition(ct prv1.ConditionType) prv1.Condition {
	return mg.Status.GetCondition(ct)
}

func (mg *ExporterScraperConfig) SetConditions(c ...prv1.Condition) {
	mg.Status.SetConditions(c...)
}

//+kubebuilder:object:root=true

// ExporterScraperConfigList contains a list of ExporterScraperConfig
type ExporterScraperConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExporterScraperConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExporterScraperConfig{}, &ExporterScraperConfigList{})
}
