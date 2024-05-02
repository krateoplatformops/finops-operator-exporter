//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExporterConfig) DeepCopyInto(out *ExporterConfig) {
	*out = *in
	if in.AdditionalVariables != nil {
		in, out := &in.AdditionalVariables, &out.AdditionalVariables
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExporterConfig.
func (in *ExporterConfig) DeepCopy() *ExporterConfig {
	if in == nil {
		return nil
	}
	out := new(ExporterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExporterScraperConfig) DeepCopyInto(out *ExporterScraperConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExporterScraperConfig.
func (in *ExporterScraperConfig) DeepCopy() *ExporterScraperConfig {
	if in == nil {
		return nil
	}
	out := new(ExporterScraperConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExporterScraperConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExporterScraperConfigList) DeepCopyInto(out *ExporterScraperConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ExporterScraperConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExporterScraperConfigList.
func (in *ExporterScraperConfigList) DeepCopy() *ExporterScraperConfigList {
	if in == nil {
		return nil
	}
	out := new(ExporterScraperConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExporterScraperConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExporterScraperConfigSpec) DeepCopyInto(out *ExporterScraperConfigSpec) {
	*out = *in
	in.ExporterConfig.DeepCopyInto(&out.ExporterConfig)
	out.ScraperConfig = in.ScraperConfig
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExporterScraperConfigSpec.
func (in *ExporterScraperConfigSpec) DeepCopy() *ExporterScraperConfigSpec {
	if in == nil {
		return nil
	}
	out := new(ExporterScraperConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExporterScraperConfigStatus) DeepCopyInto(out *ExporterScraperConfigStatus) {
	*out = *in
	out.ActiveExporter = in.ActiveExporter
	out.ConfigMap = in.ConfigMap
	out.Service = in.Service
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExporterScraperConfigStatus.
func (in *ExporterScraperConfigStatus) DeepCopy() *ExporterScraperConfigStatus {
	if in == nil {
		return nil
	}
	out := new(ExporterScraperConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScraperConfig) DeepCopyInto(out *ScraperConfig) {
	*out = *in
	out.ScraperDatabaseConfigRef = in.ScraperDatabaseConfigRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScraperConfig.
func (in *ScraperConfig) DeepCopy() *ScraperConfig {
	if in == nil {
		return nil
	}
	out := new(ScraperConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScraperConfigFromScraperOperator) DeepCopyInto(out *ScraperConfigFromScraperOperator) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScraperConfigFromScraperOperator.
func (in *ScraperConfigFromScraperOperator) DeepCopy() *ScraperConfigFromScraperOperator {
	if in == nil {
		return nil
	}
	out := new(ScraperConfigFromScraperOperator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScraperConfigSpecFromScraperOperator) DeepCopyInto(out *ScraperConfigSpecFromScraperOperator) {
	*out = *in
	out.ScraperDatabaseConfigRef = in.ScraperDatabaseConfigRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScraperConfigSpecFromScraperOperator.
func (in *ScraperConfigSpecFromScraperOperator) DeepCopy() *ScraperConfigSpecFromScraperOperator {
	if in == nil {
		return nil
	}
	out := new(ScraperConfigSpecFromScraperOperator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScraperDatabaseConfigRef) DeepCopyInto(out *ScraperDatabaseConfigRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScraperDatabaseConfigRef.
func (in *ScraperDatabaseConfigRef) DeepCopy() *ScraperDatabaseConfigRef {
	if in == nil {
		return nil
	}
	out := new(ScraperDatabaseConfigRef)
	in.DeepCopyInto(out)
	return out
}
