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
	"strings"
	"time"

	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusCollector implements the Source interface and contains all of the
// necessary metadata to query a prometheus endpoint.
type PrometheusCollector struct {
	api         promv1.API
	name        string
	resultQuery string
	errorQuery  string
	startTime   time.Time
	endTime     time.Time
}

func NewCollector(url, name, query, errorQuery string, startTime, endTime time.Time) (*PrometheusCollector, error) {
	// Nothing fancy here
	c, err := prom.NewClient(prom.Config{Address: url})
	if err != nil {
		return nil, err
	}

	return &PrometheusCollector{
		api:         promv1.NewAPI(c),
		name:        name,
		resultQuery: query,
		errorQuery:  errorQuery,
		startTime:   startTime,
		endTime:     endTime,
	}, nil
}

// This is mostly the same.. I started getting a bit carried away with refactoring because I was
// finally in front of a computer, but it should be equivalent
func (p *PrometheusCollector) Capture(ctx context.Context) (queryRes float64, errQueryRes float64, err error) {
	if !p.ready(ctx) {
		// We do run into an importing issue (cyclical) with trying to have well defined errors being used here.
		// To address the import issues
		//   - We could move this to a `internal/errors` but I'm unsure how I feel about a large errors package;
		//		 feels appropraite to be locally scoped here
		//   - We could drop CaptureError altogether and instead keep retrying within the collector since retries
		//     do not impact the remaining attempts. This would simplify the error type we return ( just a regular error )
		//     and prevent another reconcile loop.

		// TODO Can we make a more informed delay?
		return queryRes, errQueryRes, &CaptureError{RetryAfter: 5 * time.Second}
	}

	queryRes, err = p.query(ctx, p.resultQuery)
	if err != nil {
		return queryRes, errQueryRes, err
	}

	if p.errorQuery != "" {
		errQueryRes, err = p.query(ctx, p.errorQuery)
		if err != nil {
			return queryRes, errQueryRes, err
		}

	}

	return queryRes, errQueryRes, nil
}

func (p *PrometheusCollector) ready(ctx context.Context) bool {
	// Make sure Prometheus is ready
	targets, err := p.api.Targets(ctx)
	if err != nil {
		return false
	}

	for _, target := range targets.Active {
		if target.Health != promv1.HealthGood {
			continue
		}

		if target.LastScrape.Before(p.endTime) {
			return false
		}
	}

	return true
}

func (p *PrometheusCollector) query(ctx context.Context, query string) (res float64, err error) {
	// Execute query
	v, _, err := p.api.Query(ctx, query, p.endTime)
	if err != nil {
		return res, err
	}

	// Only accept scalar results
	if v.Type() != model.ValScalar {
		return res, fmt.Errorf("expected scalar query result, got %s", v.Type())
	}

	// Scalar result
	result := float64(v.(*model.Scalar).Value)
	if math.IsNaN(result) {
		err := &CaptureError{Message: "metric data not available", Address: address, Query: query, CompletionTime: completionTime}
		if strings.HasPrefix(query, "scalar(") {
			err.Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return res, err
	}

	return res, err
}
