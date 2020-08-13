package patch

import (
	"encoding/json"
	"fmt"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/template"
	"github.com/redskyops/redskyops-controller/internal/trial"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TODO: Remove as things migrate to Patcher
// RenderTemplate determines the patch target and renders the patch template
func RenderTemplate(te *template.Engine, t *redsky.Trial, p *redsky.PatchTemplate) (*corev1.ObjectReference, []byte, error) {
	// Render the actual patch data
	data, err := te.RenderPatch(p, t)
	if err != nil {
		return nil, nil, err
	}

	// Determine the reference, possibly extracting it from the rendered data
	ref := &corev1.ObjectReference{}
	switch {
	case p.TargetRef != nil:
		p.TargetRef.DeepCopyInto(ref)
	case p.Type == redsky.PatchStrategic, p.Type == "":
		m := &struct {
			metav1.TypeMeta   `json:",inline"`
			metav1.ObjectMeta `json:"metadata,omitempty"`
		}{}
		if err := json.Unmarshal(data, m); err == nil {
			ref.APIVersion = m.APIVersion
			ref.Kind = m.Kind
			ref.Name = m.Name
			ref.Namespace = m.Namespace
		}
	}

	// Default the namespace to the trial namespace
	if ref.Namespace == "" {
		ref.Namespace = t.Namespace
	}

	// Validate the reference
	if ref.Name == "" || ref.Kind == "" {
		return nil, nil, fmt.Errorf("invalid patch reference")
	}

	return ref, data, nil
}

// TODO: Remove as things migrate to Patcher
// createPatchOperation creates a new patch operation from a patch template and it's (fully rendered) patch data
func CreatePatchOperation(t *redsky.Trial, p *redsky.PatchTemplate, ref *corev1.ObjectReference, data []byte) (*redsky.PatchOperation, error) {
	po := &redsky.PatchOperation{
		TargetRef:         *ref,
		Data:              data,
		AttemptsRemaining: 3,
	}

	// If the patch is effectively null, we do not need to evaluate it
	if len(po.Data) == 0 || string(po.Data) == "null" {
		return nil, nil
	}

	// Determine the patch type
	switch p.Type {
	case redsky.PatchStrategic, "":
		po.PatchType = types.StrategicMergePatchType
	case redsky.PatchMerge:
		po.PatchType = types.MergePatchType
	case redsky.PatchJSON:
		po.PatchType = types.JSONPatchType
	default:
		return nil, fmt.Errorf("unknown patch type: %s", p.Type)
	}

	// TODO: probably should move this to the controller
	// If the patch is for the trial job itself, it cannot be applied (since the job won't exist until well after patches are applied)
	if trial.IsTrialJobReference(t, &po.TargetRef) {
		po.AttemptsRemaining = 0
		if po.PatchType != types.StrategicMergePatchType {
			return nil, fmt.Errorf("trial job patch must be a strategic merge patch")
		}
	}

	return po, nil
}
