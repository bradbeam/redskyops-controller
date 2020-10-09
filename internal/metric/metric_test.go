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

package metric

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCaptureMetric(t *testing.T) {
	// Offset all times by 10min
	// Primarily to work with remote prometheus server, but no harm in baselining
	// all tests this way
	now := metav1.NewTime(time.Now().Add(time.Duration(-10) * time.Minute))
	later := metav1.NewTime(now.Add(5 * time.Second))

	jsonHttpTest := jsonPathHttpTestServer()
	defer jsonHttpTest.Close()
	jurl, err := url.Parse(jsonHttpTest.URL)
	require.NoError(t, err)
	jsonHttpTestIP, jPort, err := net.SplitHostPort(jurl.Host)
	require.NoError(t, err)
	jsonHttpTestPort, err := strconv.ParseInt(jPort, 10, 32)
	require.NoError(t, err)

	promHttpTest := promHttpTestServer()
	defer promHttpTest.Close()
	purl, err := url.Parse(promHttpTest.URL)
	require.NoError(t, err)
	promHttpTestIP, pPort, err := net.SplitHostPort(purl.Host)
	require.NoError(t, err)
	promHttpTestPort, err := strconv.ParseInt(pPort, 10, 32)
	require.NoError(t, err)

	testCases := []struct {
		desc     string
		metric   *redskyv1beta1.Metric
		obj      runtime.Object
		expected float64
	}{
		{
			desc: "default local",
			metric: &redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
			},
			obj:      &corev1.PodList{},
			expected: 5,
		},
		{
			desc: "explicit local",
			metric: &redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "{{duration .StartTime .CompletionTime}}",
				Type:  redskyv1beta1.MetricLocal,
			},
			obj:      &corev1.PodList{},
			expected: 5,
		},
		{
			desc: "default prometheus",
			metric: &redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "scalar(prometheus_build_info)",
				Type:  redskyv1beta1.MetricPrometheus,
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: promHttpTestIP,
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     int32(promHttpTestPort),
								},
							},
						},
					},
				},
			},
			expected: 1,
		},
		{
			desc: "prometheus url",
			metric: &redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "scalar(prometheus_build_info)",
				Type:  redskyv1beta1.MetricPrometheus,
				URL:   promHttpTest.URL,
			},
			expected: 1,
		},

		{
			desc: "default jonPath",
			metric: &redskyv1beta1.Metric{
				Name:  "testMetric",
				Query: "{.current_response_time_percentile_95}",
				Type:  redskyv1beta1.MetricJSONPath,
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: jsonHttpTestIP,
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     int32(jsonHttpTestPort),
								},
							},
						},
					},
				},
			},
			expected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			trial := &redskyv1beta1.Trial{
				Status: redskyv1beta1.TrialStatus{
					StartTime:      &now,
					CompletionTime: &later,
				},
			}

			duration, _, err := CaptureMetric(tc.metric, trial, tc.obj)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, duration)
		})
	}
}

func jsonPathHttpTestServer() *httptest.Server {
	response := map[string]int{"current_response_time_percentile_95": 5}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(response)
		return
	}))
}

func promHttpTestServer() *httptest.Server {
	resp := `{"status":"success","data":{"resultType":"scalar","result":[1595471900.283,"1"]}}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, resp)
		return
	}))
}

func TestToURL(t *testing.T) {
	testCases := []struct {
		desc     string
		metric   *redskyv1beta1.Metric
		obj      *corev1.ServiceList
		expected []string
	}{
		{
			desc:   "single port",
			metric: &redskyv1beta1.Metric{},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.0.0.1",
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     9090,
								},
							},
						},
					},
				},
			},
			expected: []string{"http://10.0.0.1:9090/"},
		},

		{
			desc:   "single port headless",
			metric: &redskyv1beta1.Metric{},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "headless",
							Namespace: "default",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "None",
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     9090,
								},
							},
						},
					},
				},
			},
			expected: []string{"http://headless.default:9090/"},
		},

		{
			desc:   "multi port",
			metric: &redskyv1beta1.Metric{},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.0.0.1",
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     9090,
								},
								{
									Name:     "testPort1",
									Protocol: corev1.ProtocolTCP,
									Port:     9091,
								},
							},
						},
					},
				},
			},
			expected: []string{"http://10.0.0.1:9090/", "http://10.0.0.1:9091/"},
		},

		{
			desc: "metric port name",
			metric: &redskyv1beta1.Metric{
				Port: intstr.Parse("uberport"),
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.0.0.1",
							Ports: []corev1.ServicePort{
								{
									Name:     "uberport",
									Protocol: corev1.ProtocolTCP,
									Port:     9090,
								},
								{
									Name:     "testPort1",
									Protocol: corev1.ProtocolTCP,
									Port:     9091,
								},
							},
						},
					},
				},
			},
			expected: []string{"http://10.0.0.1:9090/"},
		},

		{
			desc: "metric port number",
			metric: &redskyv1beta1.Metric{
				Port: intstr.Parse("9090"),
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.0.0.1",
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     9090,
								},
								{
									Name:     "testPort1",
									Protocol: corev1.ProtocolTCP,
									Port:     9091,
								},
							},
						},
					},
				},
			},
			expected: []string{"http://10.0.0.1:9090/"},
		},

		{
			desc: "metric port number headless",
			metric: &redskyv1beta1.Metric{
				Port: intstr.Parse("9090"),
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "headless",
							Namespace: "default",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "None",
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort",
									Protocol: corev1.ProtocolTCP,
									Port:     9090,
								},
								{
									Name:     "testPort1",
									Protocol: corev1.ProtocolTCP,
									Port:     9091,
								},
							},
						},
					},
				},
			},
			expected: []string{"http://headless.default:9090/"},
		},

		{
			desc: "metric port name not matched with single svc port",
			metric: &redskyv1beta1.Metric{
				Port: intstr.Parse("uberport"),
			},
			obj: &corev1.ServiceList{
				Items: []corev1.Service{
					{
						Spec: corev1.ServiceSpec{
							ClusterIP: "10.0.0.1",
							Ports: []corev1.ServicePort{
								{
									Name:     "testPort1",
									Protocol: corev1.ProtocolTCP,
									Port:     9091,
								},
							},
						},
					},
				},
			},
			expected: []string{"http://10.0.0.1:9091/"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			output, err := toURL(tc.obj, tc.metric)

			assert.NoError(t, err)

			assert.Equal(t, tc.expected, output)
		})
	}
}
