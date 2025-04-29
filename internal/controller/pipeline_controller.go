package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/infraflows/loongcollector-operator/internal/emus"
	"github.com/infraflows/loongcollector-operator/internal/pkg/configserver"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infraflows/loongcollector-operator/api/v1alpha1"
)

// PipelineReconciler reconciles a Pipeline object
type PipelineReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Event   record.EventRecorder
	BaseURL string
}

const (
	defaultBaseURL     = "http://config-server:8899"
	configMapName      = "config-server-config"
	configMapNamespace = "loongcollector-system"
	configMapKey       = "configServerURL"
	maxRetries         = 3
	retryDelay         = time.Second * 5
	pipelineFinalizer  = "pipeline.finalizers.infraflow.co"
	syncInterval       = time.Minute * 5
)

// +kubebuilder:rbac:groups=infraflow.co,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infraflow.co,resources=pipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infraflow.co,resources=pipelines/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("pipeline", req.NamespacedName)

	if err := r.getConfigServerURL(ctx); err != nil {
		log.Error(err, "Failed to get ConfigServer URL")
		return reconcile.Result{}, err
	}

	pipeline := &v1alpha1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !pipeline.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(pipeline, pipelineFinalizer) {
			if err := r.cleanupPipeline(ctx, pipeline); err != nil {
				log.Error(err, "Failed to cleanup pipeline")
				return reconcile.Result{}, err
			}

			controllerutil.RemoveFinalizer(pipeline, pipelineFinalizer)
			if err := r.Update(ctx, pipeline); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(pipeline, pipelineFinalizer) {
		controllerutil.AddFinalizer(pipeline, pipelineFinalizer)
		if err := r.Update(ctx, pipeline); err != nil {
			log.Error(err, "Failed to add finalizer")
			return reconcile.Result{}, err
		}
	}

	if err := r.validatePipeline(pipeline); err != nil {
		log.Error(err, "Invalid pipeline configuration")
		pipeline.Status.Success = false
		pipeline.Status.Message = emus.PipelineStatusInvalid
		r.Event.Event(pipeline, corev1.EventTypeWarning, "InvalidPipeline", err.Error())
		pipeline.Status.LastUpdateTime = metav1.Now()
		_ = r.Status().Update(ctx, pipeline)
		return reconcile.Result{}, err
	}

	agentClient := configserver.NewAgentClient(r.BaseURL)

	// Create or update agent group if specified
	if pipeline.Spec.AgentGroup != "" {
		group := &configserver.AgentGroup{
			Name:        pipeline.Spec.AgentGroup,
			Description: fmt.Sprintf("Agent group for pipeline %s", pipeline.Name),
			Tags:        []string{pipeline.Name},
		}

		// Try to create the agent group
		if err := agentClient.CreateAgentGroup(ctx, group); err != nil {
			// If the group already exists, try to update it
			if err := agentClient.UpdateAgentGroup(ctx, group); err != nil {
				log.Error(err, "Failed to create/update agent group")
				pipeline.Status.Success = false
				pipeline.Status.Message = emus.PipelineStatusFailed
				r.Event.Event(pipeline, corev1.EventTypeWarning, "FailedToCreateAgentGroup", err.Error())
				pipeline.Status.LastUpdateTime = metav1.Now()
				_ = r.Status().Update(ctx, pipeline)
				return reconcile.Result{}, err
			}
		}
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := agentClient.ApplyPipelineToAgent(ctx, pipeline); err != nil {
			lastErr = err
			log.Error(err, "Failed to apply pipeline to agent, retrying", "attempt", i+1, "maxAttempts", maxRetries)
			time.Sleep(retryDelay)
			continue
		}

		// If agent group is specified, bind the pipeline to the group
		if pipeline.Spec.AgentGroup != "" {
			if err := agentClient.ApplyConfigToAgentGroup(ctx, pipeline.Spec.Name, pipeline.Spec.AgentGroup); err != nil {
				lastErr = err
				log.Error(err, "Failed to bind pipeline to agent group, retrying", "attempt", i+1, "maxAttempts", maxRetries)
				time.Sleep(retryDelay)
				continue
			}
		}

		lastErr = nil
		break
	}

	if lastErr != nil {
		log.Error(lastErr, "Failed to apply pipeline to agent after retries")
		pipeline.Status.Success = false
		pipeline.Status.Message = emus.PipelineStatusFailed
		r.Event.Event(pipeline, corev1.EventTypeWarning, "FailedToApplyPipeline", lastErr.Error())
		pipeline.Status.LastUpdateTime = metav1.Now()
		_ = r.Status().Update(ctx, pipeline)
		return reconcile.Result{}, lastErr
	}

	pipeline.Status.Success = true
	pipeline.Status.Message = emus.PipelineStatusSuccess
	r.Event.Event(pipeline, corev1.EventTypeNormal, "SuccessfulApplyPipeline", pipeline.Status.Message)
	pipeline.Status.LastUpdateTime = metav1.Now()
	pipeline.Status.LastAppliedConfig = v1alpha1.LastAppliedConfig{
		AppliedTime: metav1.Now(),
		Content:     pipeline.Spec.Content,
	}
	if err := r.Status().Update(ctx, pipeline); err != nil {
		log.Error(err, "Failed to update pipeline status")
		return ctrl.Result{RequeueAfter: syncInterval}, err
	}

	log.Info("Successfully applied pipeline configuration")
	return ctrl.Result{RequeueAfter: syncInterval}, nil
}

// validatePipeline validates the pipeline configuration
func (r *PipelineReconciler) validatePipeline(pipeline *v1alpha1.Pipeline) error {
	if pipeline.Spec.Name == "" {
		return fmt.Errorf("pipeline name cannot be empty")
	}
	if pipeline.Spec.Content == "" {
		return fmt.Errorf("pipeline content cannot be empty")
	}

	// Try to parse YAML to validate format
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(pipeline.Spec.Content), &config); err != nil {
		return fmt.Errorf("invalid YAML format: %v", err)
	}

	return nil
}

// getConfigServerURL gets the ConfigServer URL from ConfigMap
func (r *PipelineReconciler) getConfigServerURL(ctx context.Context) error {
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: configMapNamespace, Name: configMapName}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			r.BaseURL = defaultBaseURL
			return nil
		}
		return err
	}

	if url, ok := configMap.Data[configMapKey]; ok && url != "" {
		r.BaseURL = url
	} else {
		r.BaseURL = defaultBaseURL
	}
	return nil
}

// cleanupPipeline 清理Pipeline相关的资源
func (r *PipelineReconciler) cleanupPipeline(ctx context.Context, pipeline *v1alpha1.Pipeline) error {
	log := r.Log.WithValues("pipeline", pipeline.Name)

	agentClient := configserver.NewAgentClient(r.BaseURL)
	if err := agentClient.DeletePipelineToAgent(ctx, pipeline); err != nil {
		log.Error(err, "Failed to delete pipeline from config server")
		return err
	}

	// If agent group is specified, remove the pipeline from the group
	if pipeline.Spec.AgentGroup != "" {
		if err := agentClient.RemoveConfigFromAgentGroup(ctx, pipeline.Spec.Name, pipeline.Spec.AgentGroup); err != nil {
			log.Error(err, "Failed to remove pipeline from agent group")
			return err
		}
	}

	log.Info("Successfully cleaned up pipeline")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Pipeline{}).
		Complete(r)
}
