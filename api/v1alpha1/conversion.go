/*
Copyright 2020 GramLabs, Inc.

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

package v1alpha1

import (
	experiment "github.com/redskyops/redskyops-controller/internal/experiment"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *Experiment) ConvertTo(dstRaw conversion.Hub) error {
	//Convert_experiment_Experiment_To_v1alpha1_Experiment(src
	dst := dstRaw.(*Experiment)
	return Convert_v1alpha1_Experiment_To_experiment_Experiment(src, dst, conversion.Scope())
}

func (dst *Experiment) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*experiment.Experiment)
	return Convert_experiment_Experiment_To_v1alpha1_Experiment(src, dst, conversion.Scope())
}
