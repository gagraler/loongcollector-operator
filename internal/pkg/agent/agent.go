package agent

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/infraflows/loongcollector-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyPipelineToAgent applies a pipeline configuration to the agent
// 获取 loongcollector DaemonSet
// 为每个 pipeline 创建独立的 volume 和 volumeMount
// 将配置挂载到 loongcollector 容器的特定目录
// 1. 每个 pipeline 配置都是独立的
// 2. 配置变更会触发 pod 重启，确保配置生效
// 3. 不依赖 ConfigMap 的更新机制
func ApplyPipelineToAgent(ctx context.Context, pipeline *v1alpha1.Pipeline, client client.Client) error {
	daemonSet := &appsv1.DaemonSet{}

	// 使用 name 获取 loongcollector DaemonSet
	err := client.Get(ctx, types.NamespacedName{
		Namespace: pipeline.Namespace,
		Name:      "loongcollector-agent",
	}, daemonSet)
	if err != nil {
		return fmt.Errorf("failed to get DaemonSet: %v", err)
	}

	configVolume := corev1.Volume{
		Name: pipeline.Spec.Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: pipeline.Spec.Name,
				},
			},
		},
	}

	found := false
	for i, v := range daemonSet.Spec.Template.Spec.Volumes {
		if v.Name == pipeline.Spec.Name {
			daemonSet.Spec.Template.Spec.Volumes[i] = configVolume
			found = true
			break
		}
	}
	if !found {
		daemonSet.Spec.Template.Spec.Volumes = append(daemonSet.Spec.Template.Spec.Volumes, configVolume)
	}

	volumeMount := corev1.VolumeMount{
		Name:      pipeline.Spec.Name,
		MountPath: "/usr/local/loongcollector/conf/instance_config/" + pipeline.Spec.Name,
		ReadOnly:  true,
	}

	for i, container := range daemonSet.Spec.Template.Spec.Containers {
		if container.Name == "loongcollector" {
			found = false
			for j, vm := range container.VolumeMounts {
				if vm.Name == pipeline.Spec.Name {
					daemonSet.Spec.Template.Spec.Containers[i].VolumeMounts[j] = volumeMount
					found = true
					break
				}
			}
			if !found {
				daemonSet.Spec.Template.Spec.Containers[i].VolumeMounts = append(daemonSet.Spec.Template.Spec.Containers[i].VolumeMounts, volumeMount)
			}
			break
		}
	}

	err = client.Update(ctx, daemonSet)
	if err != nil {
		return fmt.Errorf("failed to update DaemonSet: %v", err)
	}

	return nil
}

// DeletePipelineToAgent 从 DaemonSet 中移除 Pipeline 配置
func DeletePipelineToAgent(ctx context.Context, pipeline *v1alpha1.Pipeline, client client.Client) error {
	daemonSet := &appsv1.DaemonSet{}

	// 使用 name 获取 loongcollector DaemonSet
	err := client.Get(ctx, types.NamespacedName{
		Namespace: pipeline.Namespace,
		Name:      "loongcollector-ds",
	}, daemonSet)
	if err != nil {
		return fmt.Errorf("failed to get DaemonSet: %v", err)
	}

	for i, v := range daemonSet.Spec.Template.Spec.Volumes {
		if v.Name == pipeline.Spec.Name {
			daemonSet.Spec.Template.Spec.Volumes = append(daemonSet.Spec.Template.Spec.Volumes[:i], daemonSet.Spec.Template.Spec.Volumes[i+1:]...)
			break
		}
	}

	for i, container := range daemonSet.Spec.Template.Spec.Containers {
		if container.Name == "loongcollector" {
			for j, vm := range container.VolumeMounts {
				if vm.Name == pipeline.Spec.Name {
					daemonSet.Spec.Template.Spec.Containers[i].VolumeMounts = append(container.VolumeMounts[:j], container.VolumeMounts[j+1:]...)
					break
				}
			}
			break
		}
	}

	err = client.Update(ctx, daemonSet)
	if err != nil {
		return fmt.Errorf("failed to update DaemonSet: %v", err)
	}

	return nil
}
