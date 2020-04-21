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

package config

import (
	"net/url"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// Dummy endpoint names for testing
	exp             = "/experiments/"
	expFooBar       = "/experiments/foo_bar"
	expFooBarTrials = "/experiments/foo_bar/trials/"
)

func TestRedSkyConfig_Endpoints(t *testing.T) {
	cfg := &RedSkyConfig{}
	err := defaultLoader(cfg)
	assert.NoError(t, err)

	// This is the main use case using the default data
	rss := &cfg.data.Servers[0].Server.RedSky

	cases := []struct {
		desc     string
		endpoint string
		expected []string
	}{
		{
			desc:     "default",
			endpoint: DefaultServerIdentifier,
		},
		{
			desc:     "custom_endpoint",
			endpoint: "http://example.com/api/experiments/",
		},
		{
			desc:     "no_trailing_slash",
			endpoint: "http://example.com/api/experiments",
		},
		{
			desc:     "query_params",
			endpoint: "http://example.com/api/experiments?foo=bar",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {

			rss.ExperimentsEndpoint = c.endpoint

			ep, err := cfg.Endpoints()
			assert.NoError(t, err)

			for _, testEp := range []string{exp, expFooBar, expFooBarTrials} {
				u, err := url.Parse(c.endpoint)
				assert.NoError(t, err)
				u.Path = path.Clean(path.Join(u.Path, testEp))

				assert.Equal(t, u.String(), ep.Resolve(testEp).String())
			}
		})
	}
}
