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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExporterScraperConfigSpec defines the desired state of ExporterScraperConfig
type ExporterScraperConfigSpec struct {
	// +optional
	ExporterConfig ExporterConfig `yaml:"exporterConfig" json:"exporterConfig"`
	// +optional
	ScraperConfig ScraperConfig `yaml:"scraperConfig" json:"scraperConfig"`
}

// ExporterScraperConfigStatus defines the observed state of ExporterScraperConfig
type ExporterScraperConfigStatus struct {
	// A list of pointers to currently running scraper deployments.
	ActiveExporters []corev1.ObjectReference          `json:"active,omitempty"`
	ConfigMaps      map[string]corev1.ObjectReference `json:"configMaps,omitempty"`
	Services        map[string]corev1.ObjectReference `json:"services,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ExporterScraperConfig is the Schema for the exporterscraperconfigs API
type ExporterScraperConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExporterScraperConfigSpec   `json:"spec,omitempty"`
	Status ExporterScraperConfigStatus `json:"status,omitempty"`
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

type ExporterConfig struct {
	Name                  string            `yaml:"name" json:"name"`
	URL                   string            `yaml:"url" json:"url"`
	RequireAuthentication bool              `yaml:"requireAuthentication" json:"requireAuthentication"`
	AuthenticationMethod  string            `yaml:"authenticationMethod" json:"authenticationMethod"`
	PollingIntervalHours  int               `yaml:"pollingIntervalHours" json:"pollingIntervalHours"`
	AdditionalVariables   map[string]string `yaml:"additionalVariables" json:"additionalVariables"`
}

type ScraperConfig struct {
	TableName            string `yaml:"tableName" json:"tableName"`
	PollingIntervalHours int    `yaml:"pollingIntervalHours" json:"pollingIntervalHours"`
	// +optional
	Url                      string                   `yaml:"url" json:"url"`
	ScraperDatabaseConfigRef ScraperDatabaseConfigRef `yaml:"scraperDatabaseConfigRef" json:"scraperDatabaseConfigRef"`
}

type ScraperDatabaseConfigRef struct {
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace" json:"namespace"`
}

type ScraperConfigFromScraperOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ScraperConfigSpecFromScraperOperator `json:"spec,omitempty"`
}

type ScraperConfigSpecFromScraperOperator struct {
	TableName                string                   `yaml:"tableName" json:"tableName"`
	PollingIntervalHours     int                      `yaml:"pollingIntervalHours" json:"pollingIntervalHours"`
	Url                      string                   `yaml:"url" json:"url"`
	ScraperDatabaseConfigRef ScraperDatabaseConfigRef `yaml:"scraperDatabaseConfigRef" json:"scraperDatabaseConfigRef"`
}
