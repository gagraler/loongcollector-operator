package configserver

import (
	"context"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/infraflows/loongcollector-operator/api/v1alpha1"
	"gopkg.in/yaml.v3"
)

// ConfigServerClient represents a config server client
type ConfigServerClient struct {
	client *resty.Client
}

// NewConfigServerClient creates a new config server client
func NewConfigServerClient(baseURL string) *ConfigServerClient {
	client := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(10*time.Second).
		SetHeader("Content-Type", "application/json").
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	return &ConfigServerClient{
		client: client,
	}
}

// ApplyPipelineToAgent applies a pipeline configuration to the agent
func (a *ConfigServerClient) ApplyPipelineToAgent(ctx context.Context, pipeline *v1alpha1.Pipeline) error {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(pipeline.Spec.Content), &config); err != nil {
		return fmt.Errorf("failed to parse YAML config: %v", err)
	}

	payload := map[string]interface{}{
		"config_name": pipeline.Spec.Name,
		"config_detail": map[string]interface{}{
			"name":    pipeline.Spec.Name,
			"content": config,
		},
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&response).
		Post("/User/CreateConfig")

	if err != nil {
		return fmt.Errorf("failed to send request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	if response.Code != 200 {
		return fmt.Errorf("configserver returned error: %s", response.Message)
	}

	return nil
}

// DeletePipelineToAgent 从Config-Server删除Pipeline配置
func (a *ConfigServerClient) DeletePipelineToAgent(ctx context.Context, pipeline *v1alpha1.Pipeline) error {
	resp, err := a.client.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/User/DeleteConfig/%s", pipeline.Spec.Name))

	if err != nil {
		return fmt.Errorf("failed to send delete request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 404 {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// AgentGroup represents an agent group
type AgentGroup struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CreateAgentGroup creates a new agent group
func (a *ConfigServerClient) CreateAgentGroup(ctx context.Context, group *AgentGroup) error {
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetBody(group).
		SetResult(&response).
		Post("/User/CreateAgentGroup")

	if err != nil {
		return fmt.Errorf("failed to send request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	if response.Code != 200 {
		return fmt.Errorf("configserver returned error: %s", response.Message)
	}

	return nil
}

// UpdateAgentGroup updates an existing agent group
func (a *ConfigServerClient) UpdateAgentGroup(ctx context.Context, group *AgentGroup) error {
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetBody(group).
		SetResult(&response).
		Put("/User/UpdateAgentGroup")

	if err != nil {
		return fmt.Errorf("failed to send request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	if response.Code != 200 {
		return fmt.Errorf("configserver returned error: %s", response.Message)
	}

	return nil
}

// DeleteAgentGroup deletes an agent group
func (a *ConfigServerClient) DeleteAgentGroup(ctx context.Context, groupName string) error {
	resp, err := a.client.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/User/DeleteAgentGroup/%s", groupName))

	if err != nil {
		return fmt.Errorf("failed to send delete request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 404 {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// ApplyConfigToAgentGroup applies a config to an agent group
func (a *ConfigServerClient) ApplyConfigToAgentGroup(ctx context.Context, configName, groupName string) error {
	payload := map[string]string{
		"config_name": configName,
		"group_name":  groupName,
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&response).
		Post("/User/ApplyConfigToAgentGroup")

	if err != nil {
		return fmt.Errorf("failed to send request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	if response.Code != 200 {
		return fmt.Errorf("configserver returned error: %s", response.Message)
	}

	return nil
}

// RemoveConfigFromAgentGroup removes a config from an agent group
func (a *ConfigServerClient) RemoveConfigFromAgentGroup(ctx context.Context, configName, groupName string) error {
	resp, err := a.client.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/User/RemoveConfigFromAgentGroup/%s/%s", configName, groupName))

	if err != nil {
		return fmt.Errorf("failed to send delete request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 404 {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// ListAgentGroups lists all agent groups
func (a *ConfigServerClient) ListAgentGroups(ctx context.Context) ([]AgentGroup, error) {
	var response struct {
		Code    int          `json:"code"`
		Message string       `json:"message"`
		Data    []AgentGroup `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetResult(&response).
		Get("/User/ListAgentGroups")

	if err != nil {
		return nil, fmt.Errorf("failed to send request to configserver: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("configserver returned status %d: %s", resp.StatusCode(), resp.String())
	}

	if response.Code != 200 {
		return nil, fmt.Errorf("configserver returned error: %s", response.Message)
	}

	return response.Data, nil
}
