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

package template

import (
	"testing"
	"time"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestEngine_RenderPatch(t *testing.T) {
	eng := New()

	cases := []struct {
		desc          string
		patchTemplate redskyv1beta1.PatchTemplate
		trial         redskyv1beta1.Trial
		expected      []byte
	}{
		{
			desc: "static patch",
			patchTemplate: redskyv1beta1.PatchTemplate{
				Patch: "metadata:\n  labels:\n    app: testApp\n",
				TargetRef: &corev1.ObjectReference{
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "testPod",
					APIVersion: "v1",
				},
			},
			expected: []byte(`{"metadata":{"labels":{"app":"testApp"}}}`),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual, err := eng.RenderPatch(&c.patchTemplate, &c.trial)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}

func TestEngine_RenderHelmValue(t *testing.T) {
	eng := New()

	cases := []struct {
		desc      string
		helmValue redskyv1beta1.HelmValue
		trial     redskyv1beta1.Trial
		expected  string
	}{
		{
			desc: "static string",
			helmValue: redskyv1beta1.HelmValue{
				Value: intstr.FromString("testing"),
			},
			expected: "testing",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual, err := eng.RenderHelmValue(&c.helmValue, &c.trial)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}

func TestEngine_RenderMetricQueries(t *testing.T) {
	eng := New()
	now := metav1.Now()

	cases := []struct {
		desc               string
		metric             redskyv1beta1.Metric
		trial              redskyv1beta1.Trial
		target             runtime.Object
		expectedQuery      string
		expectedErrorQuery string
	}{
		{
			desc: "function duration",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
				Type:  redskyv1beta1.MetricLocal,
			},
			trial: redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					StartTime:      &metav1.Time{Time: now.Add(-5 * time.Second)},
					CompletionTime: &now,
				},
			},
			target:        &corev1.Pod{},
			expectedQuery: "5",
		},

		{
			desc: "function percent",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "{{percent 100 5}}",
				Type:  redskyv1beta1.MetricLocal,
			},
			target:        &corev1.Pod{},
			expectedQuery: "5",
		},

		{
			desc: "function resourceRequests",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: `{{resourceRequests .Pods "cpu=0.05,memory=0.005"}}`,
				Type:  redskyv1beta1.MetricLocal,
			},
			target: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testpod1",
							Namespace: "default",
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "testContainer1",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("200m"),
											corev1.ResourceMemory: resource.MustParse("5000"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedQuery: "25010",
		},

		{
			desc: "function cpuUtilization with parameters",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: `{{cpuUtilization "component=bob,component=tom"}}`,
				Type:  redskyv1beta1.MetricLocal,
			},
			trial: redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					StartTime:      &metav1.Time{Time: now.Add(-5 * time.Second)},
					CompletionTime: &now,
				},
			},
			expectedQuery: expectedCPUUtilizationQueryWithParams,
		},

		{
			desc: "function cpuUtilization without parameters",
			metric: redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: `{{cpuUtilization}}`,
				Type:  redskyv1beta1.MetricLocal,
			},
			trial: redskyv1beta1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Status: redskyv1beta1.TrialStatus{
					StartTime:      &metav1.Time{Time: now.Add(-5 * time.Second)},
					CompletionTime: &now,
				},
			},
			expectedQuery: expectedCPUUtilizationQueryWithoutParams,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actualQuery, actualErrorQuery, err := eng.RenderMetricQueries(&c.metric, &c.trial, c.target)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expectedQuery, actualQuery)
				assert.Equal(t, c.expectedErrorQuery, actualErrorQuery)
			}
		})
	}
}

var (
	expectedCPUUtilizationQueryWithParams = `
scalar(
  sum(
    increase(container_cpu_usage_seconds_total{container="", image=""}[5s]) by (pod)
    *
    on (pod) group_left kube_pod_labels{label_component="bob",label_component="tom"}
  )
  /
  sum(
    sum_over_time(kube_pod_container_resource_limits_cpu_cores[5s:1s])
    *
    on (pod) group_left kube_pod_labels{label_component="bob",label_component="tom"}
  )
)`

	expectedCPUUtilizationQueryWithoutParams = `
scalar(
  sum(
    increase(container_cpu_usage_seconds_total{container="", image=""}[5s]) by (pod)
    *
    on (pod) group_left kube_pod_labels{namespace="default"}
  )
  /
  sum(
    sum_over_time(kube_pod_container_resource_limits_cpu_cores[5s:1s])
    *
    on (pod) group_left kube_pod_labels{namespace="default"}
  )
)`
)
