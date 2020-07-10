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

package prometheus

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	redskyapi "github.com/redskyops/redskyops-controller/api/v1beta1"
)

type PrometheusCollector struct {
	redskyapi.Metric

	API promv1.API
}

func NewPrometheusCollector(metric redskyapi.Metric) (*PrometheusCollector, error) {
	rt := prom.DefaultRoundTripper

	if metric.Proxy != "" {
		pURL, err := url.Parse(metric.Proxy)
		if err != nil {
			return nil, err
		}

		rt.(*http.Transport).Proxy = http.ProxyURL(pURL)
	}

	promClient, err := prom.NewClient(prom.Config{Address: metric.URL, RoundTripper: rt})
	if err != nil {
		return nil, err
	}

	pc := &PrometheusCollector{
		API:    promv1.NewAPI(promClient),
		Metric: metric,
	}

	return pc, nil
}

func (p *PrometheusCollector) Collect(ctx context.Context) (value float64, errValue float64, err error) {
	value, err = p.collect(ctx, p.Metric.Query)
	if err != nil {
		return value, errValue, err
	}

	errValue, err = p.collect(ctx, p.Metric.ErrorQuery)
	if err != nil {
		return value, errValue, err
	}

	return value, errValue, err
}

func (p *PrometheusCollector) collect(ctx context.Context, query string) (value float64, err error) {
	// Use empty time to fetch latest value
	// We don't care about warnings
	result, _, err := p.API.Query(ctx, query, time.Time{})
	if err != nil {
		return value, err
	}

	if result.Type() != model.ValScalar {
		return value, fmt.Errorf("expected scalar query result, got %s for query %s", result.Type(), query)
	}

	val := float64(result.(*model.Scalar).Value)

	if math.IsNaN(val) {
		err = &CaptureError{Message: "metric data not available", Query: query}
		if strings.HasPrefix(p.Metric.Query, "scalar(") {
			err.(*CaptureError).Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return value, err
	}

	return value, err
}

func (p *PrometheusCollector) Name() string {
	return ""
}
