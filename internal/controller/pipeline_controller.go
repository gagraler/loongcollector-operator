package controller

import (
	"context"
	"fmt"
	"github.com/gagraler/loongcollector-operator/internal/pkg/configserver"
	"time"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/gagraler/loongcollector-operator/api/v1"
)

// PipelineReconciler reconciles a Pipeline object
type PipelineReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
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
)

//+kubebuilder:rbac:groups=infraflow.co,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infraflow.co,resources=pipelines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infraflow.co,resources=pipelines/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("pipeline", req.NamespacedName)

	if err := r.getConfigServerURL(ctx); err != nil {
		log.Error(err, "Failed to get ConfigServer URL")
		return reconcile.Result{}, err
	}

	pipeline := &apiv1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !pipeline.ObjectMeta.DeletionTimestamp.IsZero() {
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
		pipeline.Status.Reason = fmt.Sprintf("Invalid configuration: %v", err)
		pipeline.Status.Applied = false
		_ = r.Status().Update(ctx, pipeline)
		return reconcile.Result{}, err
	}

	var lastErr error
	agentClient := configserver.NewAgentClient(r.BaseURL)
	for i := 0; i < maxRetries; i++ {
		if err := agentClient.ApplyPipelineToAgent(ctx, pipeline); err != nil {
			lastErr = err
			log.Error(err, "Failed to apply pipeline to agent, retrying", "attempt", i+1, "maxAttempts", maxRetries)
			time.Sleep(retryDelay)
			continue
		}
		lastErr = nil
		break
	}

	if lastErr != nil {
		log.Error(lastErr, "Failed to apply pipeline to agent after retries")
		pipeline.Status.Reason = fmt.Sprintf("Failed to apply: %v", lastErr)
		pipeline.Status.Applied = false
		_ = r.Status().Update(ctx, pipeline)
		return reconcile.Result{}, lastErr
	}

	pipeline.Status.Applied = true
	pipeline.Status.Reason = ""
	pipeline.Status.LastAppliedTime = metav1.Now()
	if err := r.Status().Update(ctx, pipeline); err != nil {
		log.Error(err, "Failed to update pipeline status")
		return ctrl.Result{RequeueAfter: time.Minute * 1}, err
	}

	log.Info("Successfully applied pipeline configuration")
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// validatePipeline validates the pipeline configuration
func (r *PipelineReconciler) validatePipeline(pipeline *apiv1.Pipeline) error {
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

	// Validate required fields
	if _, ok := config["inputs"]; !ok {
		return fmt.Errorf("missing required field: inputs")
	}
	if _, ok := config["outputs"]; !ok {
		return fmt.Errorf("missing required field: outputs")
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
func (r *PipelineReconciler) cleanupPipeline(ctx context.Context, pipeline *apiv1.Pipeline) error {
	log := r.Log.WithValues("pipeline", pipeline.Name)

	agentClient := configserver.NewAgentClient(r.BaseURL)
	if err := agentClient.DeletePipelineToAgent(ctx, pipeline); err != nil {
		log.Error(err, "Failed to delete pipeline from config server")
		return err
	}

	log.Info("Successfully cleaned up pipeline")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Pipeline{}).
		Complete(r)
}
