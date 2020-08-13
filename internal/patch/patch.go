package patch

import (
	"encoding/json"
	"fmt"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/template"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Patcher struct {
	Engine *template.Engine
}

func NewPatcher() *Patcher {
	return &Patcher{
		Engine: template.New(),
	}
}

func (p *Patcher) CreatePatchOperation(t *redsky.Trial, pt *redsky.PatchTemplate) (patchOp *redsky.PatchOperation, err error) {
	// Determine the patch type
	var patchType types.PatchType
	switch pt.Type {
	case redsky.PatchStrategic, "":
		patchType = types.StrategicMergePatchType
	case redsky.PatchMerge:
		patchType = types.MergePatchType
	case redsky.PatchJSON:
		patchType = types.JSONPatchType
	default:
		return nil, fmt.Errorf("unknown patch type: %s", pt.Type)
	}

	// Create rendered patch
	patchBytes, err := p.Engine.RenderPatch(pt, t)
	if err != nil {
		return nil, err
	}

	// TODO: Revisit this after the partial object meta chage in v1beta1

	patchTarget, err := createPatchTarget(patchType, patchBytes, pt.TargetRef, t.Namespace)
	if err != nil {
		return nil, err
	}

	patchOp = &redsky.PatchOperation{
		TargetRef:         *objRef,
		Data:              patchBytes,
		AttemptsRemaining: 3,
		PatchType:         patchType,
	}

	return patchOp, nil
}

func createPatchTarget(patchType types.PatchType, patchBytes []byte, targetRef *corev1.ObjectReference, trialNS string) (*metav1.PartialObjectMetadata, error) {
	// We'll honor targetRef as first priority
	// and fall back to trying to parse the targetRef
	// from the patch bytes in the case of a StrategicMergePatch.
	if targetRef == nil {
		if patchType != types.StrategicMergePatchType {
			return nil, fmt.Errorf("a target ref must be specified for the patch")
		}

		targetRef = &corev1.ObjectReference{}
	}

	// Create patch target
	ptMeta := &metav1.PartialObjectMetadata{}
	ptMeta.APIVersion = targetRef.APIVersion
	ptMeta.Kind = targetRef.Kind
	ptMeta.Name = targetRef.Name

	ptMeta.Namespace = targetRef.Namespace
	if ptMeta.Namespace == "" {
		ptMeta.Namespace = trialNS
	}

	var err error
	if err = validatePOM(ptMeta); err == nil {
		return ptMeta, nil
	}

	if err != nil && patchType != types.StrategicMergePatchType {
		return nil, err
	}

	if err = json.Unmarshal(patchBytes, ptMeta); err != nil {
		return nil, err
	}

	if err = validatePOM(ptMeta); err != nil {
		return nil, err
	}

	return ptMeta, nil
}

func validatePOM(ptMeta *metav1.PartialObjectMetadata) (err error) {
	// Verify we have a valid patch target
	switch {
	case ptMeta.Name == "":
		err = fmt.Errorf("unable to identify patch target")
	case ptMeta.APIVersion == "":
		err = fmt.Errorf("unable to identify patch target")
	case ptMeta.Name == "":
		err = fmt.Errorf("unable to identify patch target")
	case ptMeta.Namespace == "":
	}

	return err
}
