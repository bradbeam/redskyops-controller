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

	patchTarget, err := createPatchTarget(patchType, patchBytes, pt.TargetRef, t.Namespace)
	if err != nil {
		return nil, err
	}

	objRef, err := getObjectRefFromPatchTemplate(patchTarget, pt)
	if err != nil {
		fmt.Println(err)
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

// getObjectRefFromPatchTemplate constructs an corev1.ObjectReference from a patch template.
func getObjectRefFromPatchTemplate(ptMeta *metav1.PartialObjectMetadata, pt *redsky.PatchTemplate) (ref *corev1.ObjectReference, err error) {
	// Determine the reference, possibly extracting it from the rendered data
	ref = &corev1.ObjectReference{}

	switch {
	case pt.TargetRef != nil:
		pt.TargetRef.DeepCopyInto(ref)
	case pt.Type == redsky.PatchStrategic, pt.Type == "":
		ref.APIVersion = ptMeta.APIVersion
		ref.Kind = ptMeta.Kind
		ref.Name = ptMeta.Name
	default:
		return nil, fmt.Errorf("invalid patch and reference combination")
	}

	if ref.Namespace == "" {
		ref.Namespace = ptMeta.Namespace
	}

	// Validate the reference
	if ref.Name == "" || ref.Kind == "" {
		return nil, fmt.Errorf("invalid patch reference")
	}

	return ref, nil
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
