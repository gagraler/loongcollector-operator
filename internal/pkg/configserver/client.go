package configserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	apiv1 "github.com/gagraler/loongcollector-operator/api/v1alpha1"
	"gopkg.in/yaml.v3"
)

// AgentClient represents a config server client
type AgentClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAgentClient creates a new config server client
func NewAgentClient(baseURL string) *AgentClient {
	return &AgentClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ApplyPipelineToAgent applies a pipeline configuration to the agent
func (a *AgentClient) ApplyPipelineToAgent(ctx context.Context, pipeline *apiv1.Pipeline) error {
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
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/User/CreateConfig", a.baseURL), bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	cli := &http.Client{Timeout: time.Second * 10}
	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to configserver: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if response.Code != 200 {
		return fmt.Errorf("configserver returned error: %s", response.Message)
	}

	return nil
}

// DeletePipelineToAgent 从Config-Server删除Pipeline配置
func (a *AgentClient) DeletePipelineToAgent(ctx context.Context, pipeline *apiv1.Pipeline) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/User/DeleteConfig/%s", a.baseURL, pipeline.Spec.Name), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %v", err)
	}

	c := &http.Client{Timeout: time.Second * 10}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request to configserver: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("configserver returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
