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

package experiment

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestRunner(t *testing.T) {
	app, _, f := createTempApplication(t)
	defer os.Remove(f.Name())

	testCases := []struct {
		desc        string
		annotations map[string]string
	}{
		{
			desc: "presented",
		},
		{
			desc:        "confirmed",
			annotations: map[string]string{"applications.stormforge.io/make-it-happen": "number-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			appCh := make(chan *redskyappsv1alpha1.Application)
			expCh := make(chan *v1beta1.Experiment)
			errCh := make(chan error)

			runner := &Runner{
				appCh:        appCh,
				experimentCh: expCh,
				errorCh:      errCh,
			}

			ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)

			go runner.Run(ctx)

			// So we dont taint the original
			appCopy := &redskyappsv1alpha1.Application{}
			app.DeepCopyInto(appCopy)

			if tc.annotations != nil {
				appCopy.Annotations = tc.annotations
			}

			appCh <- appCopy
			select {
			case <-ctx.Done():
				assert.NoError(t, ctx.Err())
			case err := <-errCh:
				assert.NoError(t, err)
			case exp := <-expCh:
				var replicas int32 = 0
				if tc.annotations != nil {
					replicas = 1
				}
				assert.Equal(t, replicas, *exp.Spec.Replicas)
			}
		})
	}
}

// TODO look into a testing util package
func createTempApplication(t *testing.T) (*redskyappsv1alpha1.Application, []byte, *os.File) {
	tm := &metav1.TypeMeta{}
	annoyingIntPtr := 5
	annoyingDurationPtr := metav1.Duration{Duration: time.Duration(5) * time.Second}
	tm.SetGroupVersionKind(redskyappsv1alpha1.GroupVersion.WithKind("Application"))
	sampleApplication := &redskyappsv1alpha1.Application{
		TypeMeta: *tm,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sampleApplication",
			Namespace: "default",
		},
		Resources: konjure.Resources{konjure.NewResource("../../hack/nginx.yaml")},
		Parameters: &redskyappsv1alpha1.Parameters{
			ContainerResources: &redskyappsv1alpha1.ContainerResources{
				LabelSelector: "component=postgres",
			},
		},
		Scenarios: []redskyappsv1alpha1.Scenario{
			{
				Name: "how-do-you-make-a-tissue-dance",
				Locust: &redskyappsv1alpha1.LocustScenario{
					Locustfile: "../../hack/locustfile.py",
					Users:      &annoyingIntPtr,
					SpawnRate:  &annoyingIntPtr,
					RunTime:    &annoyingDurationPtr,
				},
			},
		},
		Objectives: []redskyappsv1alpha1.Objective{
			{
				Name: "cost",
				// TODO
				// line 14: cannot unmarshal !!str `100` into resource.Quantity
				// `    max: "100"`
				//Max:  resource.NewQuantity(100, resource.DecimalExponent),
				Requests: &redskyappsv1alpha1.RequestsObjective{
					MetricSelector: "everybody=yes",
					Weights: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100M"),
					},
				},
			},
		},
		Ingress: &redskyappsv1alpha1.Ingress{
			URL: "example.com",
		},
	}

	tmpfile, err := ioutil.TempFile("", "application-*.yaml")
	require.NoError(t, err)

	b, err := yaml.Marshal(sampleApplication)
	require.NoError(t, err)

	_, err = tmpfile.Write(b)
	require.NoError(t, err)

	return sampleApplication, b, tmpfile
}
