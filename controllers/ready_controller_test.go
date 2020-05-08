package controllers

import (
	"context"
	"path/filepath"
	"testing"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestReadyController(t *testing.T) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	defer testEnv.Stop()

	rtscheme := scheme.Scheme
	err = scheme.AddToScheme(rtscheme)
	require.NoError(t, err)
	err = redskyv1alpha1.AddToScheme(rtscheme)
	require.NoError(t, err)

	var k8sClient client.Client
	k8sClient, err = client.New(cfg, client.Options{Scheme: rtscheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	// Verify CRD was added
	for _, obj := range []runtime.Object{&redskyv1alpha1.ExperimentList{}, &redskyv1alpha1.TrialList{}} {
		err = k8sClient.List(context.Background(), obj)
		require.NoError(t, err)
		require.NotNil(t, obj)
	}

	tmr := &TestReadyReconciler{
		ReadyReconciler: ReadyReconciler{
			Client:    k8sClient,
			Scheme:    rtscheme,
			Log:       ctrl.Log,
			apiReader: k8sClient,
		},
	}

	testCases := map[string]func(t *testing.T){"checkReadiness": tmr.tester}

	for name, testCase := range testCases {
		t.Run(name, testCase)
	}
}

type TestReadyReconciler struct {
	ReadyReconciler
}

func (r *TestReadyReconciler) tester(t *testing.T) {
	experiment, trial, pod := r.createExperimentAndTrialAndPod()

	resources := []runtime.Object{experiment, trial, pod}

	// Expose trial status conditions for test cases
	// Expose trial readiness gates for tests

	var err error

	for _, obj := range resources {
		err = r.Create(context.Background(), obj)
		assert.NoError(t, err)
	}

	now := metav1.Now()
	_, err = r.evaluateReadinessChecks(context.Background(), trial, &now)
	assert.NoError(t, err)

	res, err := r.checkReadiness(context.Background(), trial, &now)
	assert.NoError(t, err)

	/*
		for _, condition := range trial.Status.Conditions {
			if condition.Type != redskyv1alpha1.TrialReady {
				continue
			}
			assert.Equal(t, corev1.ConditionTrue, condition.Status)
		}

	*/
	t.Logf("%+v", trial)
	t.Log(res)
}

func (r *TestReadyReconciler) createExperimentAndTrialAndPod() (*redskyv1alpha1.Experiment, *redskyv1alpha1.Trial, *corev1.Pod) {
	// Create Experiment
	exp := &redskyv1alpha1.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "myexperiment",
		},
		Spec: redskyv1alpha1.ExperimentSpec{
			Metrics: []redskyv1alpha1.Metric{
				{
					Name:  "testMetric",
					Query: "{{duration .StartTime .CompletionTime}}",
					Type:  redskyv1alpha1.MetricLocal,
				},
			},
		},
	}
	// Create Trial
	tri := &redskyv1alpha1.Trial{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "myexperiment",
		},
		Spec: redskyv1alpha1.TrialSpec{
			Values: []redskyv1alpha1.Value{
				{
					Name:              "testMetric",
					Value:             "123",
					AttemptsRemaining: 0,
				},
			},
			ReadinessGates: []redskyv1alpha1.TrialReadinessGate{
				{
					Kind: "Pod",
					Name: "mypod",
					ConditionTypes: []string{
						"Ready",
					},
				},
			},
		},
		/*
			Status: redskyv1alpha1.TrialStatus{
				Conditions: []redskyv1alpha1.TrialCondition{
					{
						Type:               redskyv1alpha1.TrialReady,
						Status:             corev1.ConditionFalse,
						LastProbeTime:      metav1.Now(),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
		*/
	}

	// Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "mypod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "web",
					Image: "nginx:1.12",
				},
			},
		},
	}

	return exp, tri, pod
}

func (r *TestReadyReconciler) cleanup(t *testing.T, objs ...runtime.Object) {
	for _, obj := range objs {
		err := r.Delete(context.Background(), obj)
		assert.NoError(t, err)
	}
}
