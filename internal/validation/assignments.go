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

package validation

import redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"

// AssignmentError is raised when trial assignments do not match the experiment parameter definitions
type AssignmentError struct {
	// Parameter names for which the assignment is missing
	Unassigned []string
	// Parameter names for which there is no definition
	Undefined []string
	// Parameter names for which the assignment is out of bounds
	OutOfBounds []string
	// Parameter names for which multiple assignments exist
	Duplicated []string
}

// Error returns a message describing the nature of the problems with the assignments
func (e *AssignmentError) Error() string {
	// TODO Improve this error message
	return "invalid assignments"
}

// CheckAssignments ensures the trial assignments match the definitions on the experiment
func CheckAssignments(t *redskyv1beta1.Trial, exp *redskyv1beta1.Experiment) error {
	err := &AssignmentError{}

	// Index the assignments, checking for duplicates
	assignments := make(map[string]int64, len(t.Spec.Assignments))
	for _, a := range t.Spec.Assignments {
		if _, ok := assignments[a.Name]; !ok {
			assignments[a.Name] = a.Value
		} else {
			err.Duplicated = append(err.Duplicated, a.Name)
		}
	}

	// Verify against the parameter specifications
	for _, p := range exp.Spec.Parameters {
		if a, ok := assignments[p.Name]; ok {
			if a < p.Min || a > p.Max {
				err.OutOfBounds = append(err.OutOfBounds, p.Name)
			}
			delete(assignments, p.Name)
		} else {
			err.Unassigned = append(err.Unassigned, p.Name)
		}
	}
	for n := range assignments {
		err.Undefined = append(err.Undefined, n)
	}

	// If there were no problems found, return nil
	if len(err.Unassigned) == 0 && len(err.Undefined) == 0 && len(err.OutOfBounds) == 0 && len(err.Duplicated) == 0 {
		return nil
	}
	return err
}
