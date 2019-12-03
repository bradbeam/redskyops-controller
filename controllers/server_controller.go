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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/redskyops/k8s-experiment/internal/controller"
	"github.com/redskyops/k8s-experiment/internal/meta"
	"github.com/redskyops/k8s-experiment/internal/server"
	"github.com/redskyops/k8s-experiment/internal/trial"
	redskyapi "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/experiment"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ServerReconciler reconciles a experiment and trial objects with a remote server
type ServerReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	RedSkyAPI redskyapi.API
}

func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if _, err := r.RedSkyAPI.Options(context.Background()); err != nil {
		// TODO We may need to ignore transient errors to prevent skipping setup in recoverable or "not ready" scenarios
		// TODO We may need to look for specific errors to skip setup, i.e. "ErrConfigAddressMissing"
		r.Log.Info("Red Sky API is unavailable, skipping setup", "message", err.Error())
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&redskyv1alpha1.Experiment{}).
		Owns(&redskyv1alpha1.Trial{}).
		Complete(r)
}

// TODO Update RBAC

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list

func (r *ServerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("experiment", req.NamespacedName)

	// Fetch the experiment state from the cluster
	exp := &redskyv1alpha1.Experiment{}
	if err := r.Get(ctx, req.NamespacedName, exp); err != nil {
		return ctrl.Result{}, controller.IgnoreNotFound(err)
	}

	// Create the experiment on the server
	if exp.GetReplicas() > 0 {
		if result, err := r.createExperiment(ctx, log, exp); result != nil {
			return *result, err
		}
	}

	// Create a new trial if necessary
	if exp.Status.ActiveTrials < exp.GetReplicas() {
		if result, err := r.nextTrial(ctx, log, exp); result != nil {
			return *result, err
		}
	}

	// Check each trial
	trialList, err := r.listTrials(ctx, exp)
	if err != nil {
		return ctrl.Result{}, err
	}
	var trialHasFinalizer bool
	for i := range trialList.Items {
		t := &trialList.Items[i]
		if trial.IsFinished(t) {
			if result, err := r.reportTrial(ctx, log, t); result != nil {
				return *result, err
			}
		} else if !t.GetDeletionTimestamp().IsZero() {
			if result, err := r.abandonTrial(ctx, t); result != nil {
				return *result, err
			}
		}

		// We cannot delete the experiment if any trial still has a finalizer
		// TODO Instead of a boolean should we use an array of trial names and make a useful log message?
		trialHasFinalizer = trialHasFinalizer || meta.HasFinalizer(t, server.Finalizer)
	}

	// Delete the experiment
	if !exp.DeletionTimestamp.IsZero() && !trialHasFinalizer {
		if result, err := r.deleteExperiment(ctx, exp); result != nil {
			return *result, err
		}
	}

	// Nothing to do
	return ctrl.Result{}, nil
}

// createExperiment will create a new experiment on the server using the cluster state; any default values from the
// server will be copied back into cluster along with the URLs needed for future interactions with server.
func (r *ServerReconciler) createExperiment(ctx context.Context, log logr.Logger, exp *redskyv1alpha1.Experiment) (*ctrl.Result, error) {
	// Check if we have already created the experiment
	if exp.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL] != "" {
		return nil, nil
	}

	// Convert the cluster state into a server representation
	n, e := server.FromCluster(exp)

	log.Info("Creating remote experiment")
	ee, err := r.RedSkyAPI.CreateExperiment(ctx, n, *e)
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Apply the server response to the cluster state
	server.ToCluster(exp, &ee)

	// Add a finalizer so the experiment cannot be deleted without first updating the server
	meta.AddFinalizer(exp, server.Finalizer)
	err = r.Update(ctx, exp)
	return controller.RequeueConflict(err)
}

// deleteExperiment will delete the experiment from the server using the URLs recorded in the cluster; the finalizer
// added when the experiment was created on the server will also be removed
func (r *ServerReconciler) deleteExperiment(ctx context.Context, exp *redskyv1alpha1.Experiment) (*ctrl.Result, error) {
	// Try to remove the finalizer, if it is already gone we do not need to do anything
	if !meta.RemoveFinalizer(exp, server.Finalizer) {
		return nil, nil
	}

	// Delete the experiment from the server if we still have a URL
	if experimentURL := exp.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL]; experimentURL != "" {
		err := r.RedSkyAPI.DeleteExperiment(ctx, experimentURL)
		if controller.IgnoreNotFound(err) != nil {
			return &ctrl.Result{}, err
		}

		delete(exp.GetAnnotations(), redskyv1alpha1.AnnotationExperimentURL)
		delete(exp.GetAnnotations(), redskyv1alpha1.AnnotationNextTrialURL)
	}

	// This update will include the removal of the finalizer and URL annotations
	err := r.Update(ctx, exp)
	return controller.RequeueConflict(err)
}

// nextTrial will try to obtain a suggestion from the server and create the corresponding cluster state in the form of
// a trial; if the cluster can not accommodate additional trials at the time of invocation, not action will be taken
func (r *ServerReconciler) nextTrial(ctx context.Context, log logr.Logger, exp *redskyv1alpha1.Experiment) (*ctrl.Result, error) {
	// Check if we have an endpoint to obtain trials from
	nextTrialURL := exp.GetAnnotations()[redskyv1alpha1.AnnotationNextTrialURL]
	if nextTrialURL == "" {
		return nil, nil
	}

	// Determine the namespace (if any) to use for the trial
	// TODO This logic needs to change so we are creating namespaces
	trialList, err := r.listTrials(ctx, exp)
	if err != nil {
		return &ctrl.Result{}, err
	}
	namespace, err := experiment.FindAvailableNamespace(r, exp, trialList.Items)
	if err != nil {
		return &ctrl.Result{}, err
	}
	if namespace == "" {
		return nil, nil
	}

	// Create a new trial from the template on the experiment
	t := &redskyv1alpha1.Trial{}
	experiment.PopulateTrialFromTemplate(exp, t, namespace)
	if err := controllerutil.SetControllerReference(exp, t, r.Scheme); err != nil {
		return &ctrl.Result{}, err
	}

	// Obtain a suggestion from the server
	suggestion, err := r.RedSkyAPI.NextTrial(ctx, nextTrialURL)
	if err != nil {
		if server.StopExperiment(exp, err) {
			err := r.Update(ctx, exp)
			return controller.RequeueConflict(err)
		}
		return controller.RequeueIfUnavailable(err)
	}

	log.Info("Creating new trial", "namespace", t.Namespace, "reportTrialURL", suggestion.ReportTrial, "assignments", t.Spec.Assignments)

	// Apply the server response to the cluster state
	server.ToClusterTrial(t, &suggestion)

	// Add a finalizer so the trial cannot be deleted without first updating the server
	meta.AddFinalizer(t, server.Finalizer)
	err = r.Create(ctx, t)
	// TODO If there is an error, notify server that we failed to adopt the suggestion?
	return &ctrl.Result{}, err
}

// reportTrial will report the values from a finished in cluster trial back to the server
func (r *ServerReconciler) reportTrial(ctx context.Context, log logr.Logger, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	// If the report URL has been removed, make sure the finalizer is also removed
	reportTrialURL := t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL]
	if reportTrialURL == "" {
		if meta.RemoveFinalizer(t, server.Finalizer) {
			err := r.Update(ctx, t)
			return controller.RequeueConflict(err)
		}
		return nil, nil
	}

	// NOTE: Because the server operation is not idempotent, the order of operations is different here then in other
	// places in the code: i.e. we do the Kube API update *first* before trying to update the server (we are more likely
	// to conflict in the Kube API); this also means that returning an empty `ctrl.Result` will still result in an
	// immediate call back into the reconciliation logic since we *do not* return from a successful Kube API update.

	// Remove the report trial URL from the trial before updating the server
	delete(t.GetAnnotations(), redskyv1alpha1.AnnotationReportTrialURL)
	if err := r.Update(ctx, t); err != nil {
		return controller.RequeueConflict(err)
	}

	// Create an observation for the remote server and log it
	trialValues := server.FromClusterTrial(t)
	log = loggerWithConditions(log, &t.Status)
	log.Info("Reporting trial", "namespace", t.Namespace, "reportTrialURL", reportTrialURL, "assignments", t.Spec.Assignments, "values", trialValues)

	// Send the observation to the server
	err := r.RedSkyAPI.ReportTrial(ctx, reportTrialURL, *trialValues)
	// TODO Restore `reportTrialURL` annotation to retry on error?
	return &ctrl.Result{}, controller.IgnoreNotFound(err)
}

// abandonTrial will remove the finalizer and try to notify the server that the trial will not be reported
func (r *ServerReconciler) abandonTrial(ctx context.Context, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	if !meta.RemoveFinalizer(t, server.Finalizer) {
		return nil, nil
	}

	if reportTrialURL := t.GetAnnotations()[redskyv1alpha1.AnnotationReportTrialURL]; reportTrialURL != "" {
		if err := r.RedSkyAPI.AbandonRunningTrial(ctx, reportTrialURL); controller.IgnoreNotFound(err) != nil {
			return &ctrl.Result{}, err
		}
		delete(t.GetAnnotations(), redskyv1alpha1.AnnotationReportTrialURL)
	}

	err := r.Update(ctx, t)
	return controller.RequeueConflict(err)
}

// listTrials will return all of the in cluster trials for the experiment
func (r *ServerReconciler) listTrials(ctx context.Context, exp *redskyv1alpha1.Experiment) (*redskyv1alpha1.TrialList, error) {
	trialList := &redskyv1alpha1.TrialList{}
	matchingSelector, err := meta.MatchingSelector(exp.GetTrialSelector())
	if err != nil {
		return nil, err
	}
	if err := r.List(ctx, trialList, matchingSelector); err != nil {
		return nil, err
	}
	return trialList, nil
}

// logWithConditions returns a logger with additional key/value pairs extracted from the trial status
func loggerWithConditions(log logr.Logger, s *redskyv1alpha1.TrialStatus) logr.Logger {
	// TODO Should this just iterate over the conditions and include all non-empty reason/messages?
	for i := range s.Conditions {
		c := s.Conditions[i]
		if c.Type == redskyv1alpha1.TrialFailed && c.Status == corev1.ConditionTrue {
			log = log.WithValues("failureReason", c.Reason, "failureMessage", c.Message)
		}
	}
	return log
}
