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

package trial

import (
	"strings"

	"github.com/redskyops/redskyops-controller/internal/hub"
)

// GetInitializers returns the initializers for the specified trial
func GetInitializers(t *hub.Trial) []string {
	var initializers []string
	for _, e := range strings.Split(t.GetAnnotations()[hub.AnnotationInitializer], ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			initializers = append(initializers, e)
		}
	}
	return initializers
}

// SetInitializers sets the supplied initializers on the trial
func SetInitializers(t *hub.Trial, initializers []string) {
	a := t.GetAnnotations()
	if a == nil {
		a = make(map[string]string, 1)
	}
	if len(initializers) > 0 {
		a[hub.AnnotationInitializer] = strings.Join(initializers, ",")
	} else {
		delete(a, hub.AnnotationInitializer)
	}
	t.SetAnnotations(a)
}

// AddInitializer adds an initializer to the trial; returns true only if the trial is changed
func AddInitializer(t *hub.Trial, initializer string) bool {
	init := GetInitializers(t)
	for _, e := range init {
		if e == initializer {
			return false
		}
	}
	SetInitializers(t, append(init, initializer))
	return true
}

// RemoveInitializer removes the first occurrence of an initializer from the trial; returns true only if the trial is changed
func RemoveInitializer(t *hub.Trial, initializer string) bool {
	init := GetInitializers(t)
	for i, e := range init {
		if e == initializer {
			SetInitializers(t, append(init[:i], init[i+1:]...))
			return true
		}
	}
	return false
}
