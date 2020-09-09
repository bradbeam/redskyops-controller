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
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// FuncMap returns the functions used for template evaluation
func FuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	extra := template.FuncMap{
		"duration":         duration,
		"percent":          percent,
		"resourceRequests": resourceRequests,
		"promServer":       promServer,
		"rsoTargetLabel":   rsoTargetLabel,
	}

	for k, v := range extra {
		f[k] = v
	}

	return f
}

// duration returns a floating point number representing the number of seconds between two times
func duration(start, completion time.Time) float64 {
	if start.Before(completion) {
		return completion.Sub(start).Seconds()
	}
	return 0
}

// percent returns a percentage of an integer value using an integer (0-100) percentage
func percent(value int64, percent int64) string {
	return fmt.Sprintf("%d", int64(float64(value)*(float64(percent)/100.0)))
}

// resourceRequests uses a map of resource types to weights to calculate a weighted sum of the resource requests
func resourceRequests(pods corev1.PodList, weights string) (float64, error) {
	var totalResources float64
	parsedWeights := make(map[string]float64)

	for _, singleEntry := range strings.Split(weights, ",") {
		parsedEntry := strings.Split(singleEntry, "=")
		weight, err := strconv.ParseFloat(parsedEntry[1], 64)
		if err != nil {
			return 0.0, fmt.Errorf("unable to parse weight for %s", parsedEntry[0])
		}
		parsedWeights[parsedEntry[0]] = weight
	}
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			for resourceType, weight := range parsedWeights {
				resourceValue := container.Resources.Requests[corev1.ResourceName(resourceType)]
				totalResources += weight * float64(resourceValue.MilliValue())
			}
		}
	}
	return totalResources, nil
}

const PrometheusServiceName = "rso-prometheus"

func promServer(expName types.NamespacedName) string {
	return fmt.Sprintf("%s.%s", PrometheusServiceName, expName.Namespace)
}

func rsoTargetLabel(tm metav1.ObjectMeta) string {

	labelPair, ok := tm.Annotations[redskyv1beta1.AnnotationMetricTarget]
	if !ok {
		return ""
	}

	labels := strings.Split(labelPair, "=")
	if len(labels) != 2 {
		return ""
	}

	return fmt.Sprintf("{label_%s=\"%s\"}", labels[0], labels[1])
}
