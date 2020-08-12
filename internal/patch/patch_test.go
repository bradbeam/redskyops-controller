package patch

import (
	"fmt"
	"testing"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test(t *testing.T) {
	patcher := NewPatcher()

	trial := &redsky.Trial{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mytrial",
			Namespace: "default",
		},
	}

	patch := `spec:
        template:
          spec:
            containers:
            - name: postgres
              imagePullPolicy: Always`

	testCases := []struct {
		desc          string
		trial         *redsky.Trial
		patchTemplate *redsky.PatchTemplate
		expectedError bool
	}{
		{
			desc:  "default",
			trial: trial,
			patchTemplate: &redsky.PatchTemplate{
				// Note, defining an empty string ("") results
				// in a `null` being returned
				Patch: patch,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       "myapp",
					Namespace:  "default",
				},
			},
		},
		{
			desc:  "strategic w/o targetref",
			trial: trial,
			patchTemplate: &redsky.PatchTemplate{
				Type: redsky.PatchStrategic,
				// Note, defining an empty string ("") results
				// in a `null` being returned
				Patch: patch,
				// nil panic
				TargetRef: nil,
			},
		},
		{
			desc:  "patchmerge",
			trial: trial,
			patchTemplate: &redsky.PatchTemplate{
				Type: redsky.PatchMerge,
				// Note, defining an empty string ("") results
				// in a `null` being returned
				Patch: patch,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       "myapp",
					Namespace:  "default",
				},
			},
		},
		{
			desc:  "patchjson",
			trial: trial,
			patchTemplate: &redsky.PatchTemplate{
				Type: redsky.PatchJSON,
				// Note, defining an empty string ("") results
				// in a `null` being returned
				Patch: patch,
				TargetRef: &corev1.ObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       "myapp",
					Namespace:  "default",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			op, err := patcher.CreatePatchOperation(tc.trial, tc.patchTemplate)
			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			assert.NotNil(t, op)

			// Cant check equality here since they are different
			//assert.Equal(t, tc.patchTemplate.Type, op.PatchType)
			if tc.patchTemplate.TargetRef != nil {
				assert.Equal(t, *tc.patchTemplate.TargetRef, op.TargetRef)
			}

			if op != nil {
				assert.Contains(t, string(op.Data), "Always")
			}
		})
	}
}

// TODO
// redsky.PatchStrategic
// redsky.PatchMerge
// redsky.PatchJSON

// targetRef optional
// ReadinessGates []redsky.PatchReadinessGate{ { ConditionType: string } }
/*
  - targetRef:
      kind: Deployment
      apiVersion: apps/v1
      name: postgres
    patch: |
      spec:
        template:
          spec:
            containers:
            - name: postgres
              resources:
                limits:
                  cpu: "{{ .Values.cpu }}m"
                  memory: "{{ .Values.memory }}Mi"
                requests:
                  cpu: "{{ .Values.cpu }}m"
                  memory: "{{ .Values.memory }}Mi"
*/
