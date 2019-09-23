/*
Copyright 2019 GramLabs, Inc.

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
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsTrialFinished checks to see if the specified trial is finished
func IsTrialFinished(trial *redskyv1alpha1.Trial) bool {
	for _, c := range trial.Status.Conditions {
		if (c.Type == redskyv1alpha1.TrialComplete || c.Type == redskyv1alpha1.TrialFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// IsTrialActive checks to see if the specified trial and any setup delete tasks are NOT finished
func IsTrialActive(trial *redskyv1alpha1.Trial) bool {
	if !IsTrialFinished(trial) {
		return true
	}
	for _, c := range trial.Status.Conditions {
		if c.Type == redskyv1alpha1.TrialSetupDeleted && c.Status != corev1.ConditionTrue {
			// A setup delete tasks exists and has not yet completed (remember the TrialSetupDeleted status is optional!)
			return true
		}
	}
	return false
}

// ApplyCondition updates a the status of an existing condition or adds it if it does not exist
func ApplyCondition(status *redskyv1alpha1.TrialStatus, conditionType redskyv1alpha1.TrialConditionType, conditionStatus corev1.ConditionStatus, reason, message string, time *metav1.Time) {
	if time == nil {
		now := metav1.Now()
		time = &now
	}

	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			if status.Conditions[i].Status != conditionStatus {
				status.Conditions[i].Status = conditionStatus
				status.Conditions[i].Reason = reason
				status.Conditions[i].Message = message
				status.Conditions[i].LastTransitionTime = *time
			} else {
				status.Conditions[i].LastProbeTime = *time
				if status.Conditions[i].Reason != reason {
					status.Conditions[i].Reason = reason
					status.Conditions[i].Message = message
				}
			}
			return
		}
	}

	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      *time,
		LastTransitionTime: *time,
	})
}

// CheckCondition checks to see if a condition has a specific status and if it exists
func CheckCondition(status *redskyv1alpha1.TrialStatus, conditionType redskyv1alpha1.TrialConditionType, conditionStatus corev1.ConditionStatus) (bool, bool) {
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return status.Conditions[i].Status == conditionStatus, true
		}
	}
	return false, false
}
