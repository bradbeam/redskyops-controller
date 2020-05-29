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
	"testing"

	"github.com/redskyops/redskyops-controller/internal/hub"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestUpdateStatus_Summarize(t *testing.T) {
	cases := []struct {
		desc       string
		conditions []hub.TrialCondition
		phase      string
	}{
		{
			desc:  "Created",
			phase: created,
		},
		{
			desc: "HasSetupTasks",
			conditions: []hub.TrialCondition{
				{
					Type:   hub.TrialSetupCreated,
					Status: corev1.ConditionUnknown,
				},
				{
					Type:   hub.TrialSetupDeleted,
					Status: corev1.ConditionUnknown,
				},
			},
			phase: settingUp,
		},
		{
			desc: "SettingUp",
			conditions: []hub.TrialCondition{
				{
					Type:   hub.TrialSetupCreated,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   hub.TrialSetupDeleted,
					Status: corev1.ConditionUnknown,
				},
			},
			phase: settingUp,
		},
		{
			desc: "SetupCreated",
			conditions: []hub.TrialCondition{
				{
					Type:   hub.TrialSetupCreated,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   hub.TrialSetupDeleted,
					Status: corev1.ConditionUnknown,
				},
			},
			phase: setupCreated,
		},
		{
			desc: "SetupCreateFailure",
			conditions: []hub.TrialCondition{
				{
					Type:   hub.TrialSetupCreated,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   hub.TrialSetupDeleted,
					Status: corev1.ConditionUnknown,
				},
				{
					Type:   hub.TrialFailed,
					Status: corev1.ConditionTrue,
				},
			},
			phase: failed,
		},
		{
			desc: "SetupCreateUnexpectedFailure",
			conditions: []hub.TrialCondition{
				{
					Type:   hub.TrialSetupCreated,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   hub.TrialSetupDeleted,
					Status: corev1.ConditionUnknown,
				},
				{
					Type:   hub.TrialFailed,
					Status: corev1.ConditionTrue,
				},
			},
			phase: failed,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			tt := &hub.Trial{Status: hub.TrialStatus{Conditions: c.conditions}}
			UpdateStatus(tt)
			assert.Equal(t, c.phase, tt.Status.Phase)
		})
	}
}

func TestUpdateStatus_Values(t *testing.T) {
	cases := []struct {
		desc       string
		conditions []hub.TrialCondition
		values     []hub.Value
		value      string
	}{
		{
			desc: "OneValue",
			values: []hub.Value{
				{
					Name:  "foo",
					Value: "1.0",
				},
			},
			value: "foo=1.0",
		},
		{
			desc: "TwoValues",
			values: []hub.Value{
				{
					Name:  "foo",
					Value: "1.0",
				},
				{
					Name:  "bar",
					Value: "2.0",
				},
			},
			value: "foo=1.0, bar=2.0",
		},
		{
			desc: "NotReady",
			values: []hub.Value{
				{
					Name:              "foo",
					Value:             "1.0",
					AttemptsRemaining: 1,
				},
				{
					Name:  "bar",
					Value: "2.0",
				},
			},
			value: "bar=2.0",
		},
		{
			desc: "NoneReady",
			values: []hub.Value{
				{
					Name:              "foo",
					Value:             "1.0",
					AttemptsRemaining: 1,
				},
				{
					Name:              "bar",
					Value:             "2.0",
					AttemptsRemaining: 1,
				},
			},
			value: "",
		},
		{
			desc: "Failed",
			conditions: []hub.TrialCondition{
				{
					Type:    hub.TrialFailed,
					Status:  corev1.ConditionTrue,
					Message: "test failure message",
				},
			},
			value: "test failure message",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			tt := &hub.Trial{
				Spec:   hub.TrialSpec{Values: c.values},
				Status: hub.TrialStatus{Conditions: c.conditions},
			}
			UpdateStatus(tt)
			assert.Equal(t, c.value, tt.Status.Values)
		})
	}
}
