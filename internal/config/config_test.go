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
	"fmt"
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

func TestRedSkyConfig_Load(t *testing.T) {
	defaultConfig := &RedSkyConfig{}
	err := defaultConfig.Load()
	assert.NoError(t, err)

	doNothingLoader := func(cfg *RedSkyConfig) error {
		return nil
	}
	errLoader := func(cfg *RedSkyConfig) error {
		return fmt.Errorf("expected error")
	}

	changedConfig := &RedSkyConfig{
		Filename: "yolo",
	}
	err = changedConfig.Load()
	assert.NoError(t, err)

	changedLoader := func(cfg *RedSkyConfig) error {
		cfg.Filename = "yolo"
		return nil
	}

	cases := []struct {
		desc          string
		loaders       []Loader
		cfg           *RedSkyConfig
		expectedError bool
	}{
		{
			desc:          "no_extras",
			loaders:       []Loader{},
			cfg:           defaultConfig,
			expectedError: false,
		},
		{
			desc: "simple",
			loaders: []Loader{
				doNothingLoader,
			},
			cfg:           defaultConfig,
			expectedError: false,
		},
		{
			desc: "error",
			loaders: []Loader{
				doNothingLoader,
				errLoader,
			},
			cfg:           defaultConfig,
			expectedError: true,
		},
		{
			desc: "changed",
			loaders: []Loader{
				changedLoader,
			},
			cfg:           changedConfig,
			expectedError: false,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			cfg := &RedSkyConfig{}
			err := cfg.Load(c.loaders...)
			if c.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.cfg, cfg)
		})
	}
}

func TestRedSkyConfig_Update(t *testing.T) {
	defaultConfig := &RedSkyConfig{}
	err := defaultConfig.Load()
	assert.NoError(t, err)

	doNothingChange := func(cfg *Config) error {
		return nil
	}

	errChange := func(cfg *Config) error {
		return fmt.Errorf("expected error")
	}

	cases := []struct {
		desc          string
		change        Change
		expectedError bool
	}{
		{
			desc:          "no_change",
			change:        doNothingChange,
			expectedError: false,
		},
		{
			desc:          "err_change",
			change:        errChange,
			expectedError: true,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			cfg := &RedSkyConfig{}
			err = cfg.Update(c.change)
			if c.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, cfg.unpersisted, 1)
		})
	}
}
