package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mosoteach/internal/config"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	systemPrompt = `你是一个专业的答题助手。请直接给出答案，不需要解释过程。对于选择题，只需要给出答案的选项字母（如A、B、C、D）。对于判断题，只需要回答"正确"或"错误"。对于填空题，直接给出答案内容。`

	httpTimeout = 60 * time.Second
)

var (
	sharedHTTPClient *http.Client
	httpClientOnce   sync.Once
)

// getHTTPClient 获取共享的 HTTP 客户端
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		sharedHTTPClient = &http.Client{
			Timeout: httpTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	})
	return sharedHTTPClient
}

// ChatRequest OpenAI API 请求结构
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// UnifiedModel 统一模型（支持所有OpenAI兼容API）
type UnifiedModel struct {
	cfg config.ModelConfig
}

// NewUnifiedModel 创建统一模型
func NewUnifiedModel(cfg config.ModelConfig) *UnifiedModel {
	return &UnifiedModel{
		cfg: cfg,
	}
}

// GetAnswer 获取答案
func (m *UnifiedModel) GetAnswer(ctx context.Context, question string) (string, error) {
	if question == "" {
		return "", fmt.Errorf("题目内容为空")
	}

	// 构建请求
	reqBody := ChatRequest{
		Model: m.cfg.Model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("下面是一道题目:%s", question),
			},
		},
		Temperature: 0.1,
		MaxTokens:   1000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建URL
	// 用户可以配置完整路径（如 https://api.example.com/v1/chat/completions）
	// 或者只配置基础URL（如 https://api.deepseek.com），代码会自动补全
	baseURL := strings.TrimSuffix(m.cfg.BaseURL, "/")
	var url string
	if strings.HasSuffix(baseURL, "/chat/completions") {
		// 用户已配置完整路径
		url = baseURL
	} else if strings.Contains(baseURL, "/v1") || strings.Contains(baseURL, "/v1beta") {
		// URL已包含版本路径，只需添加 /chat/completions
		url = baseURL + "/chat/completions"
	} else {
		// 添加默认的 /v1/chat/completions
		url = baseURL + "/v1/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)

	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w, body: %s", err, string(body))
	}

	// 检查错误
	if chatResp.Error != nil {
		return "", fmt.Errorf("API错误: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("没有返回答案")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// Name 获取模型名称
func (m *UnifiedModel) Name() string {
	return m.cfg.Name
}

// ModelManager 模型管理器
type ModelManager struct {
	models []*UnifiedModel
}

// NewModelManager 创建模型管理器
func NewModelManager() *ModelManager {
	cfg := config.GetConfig()
	enabledModels := cfg.GetEnabledModels()

	manager := &ModelManager{
		models: make([]*UnifiedModel, 0, len(enabledModels)),
	}

	for _, modelCfg := range enabledModels {
		manager.models = append(manager.models, NewUnifiedModel(modelCfg))
	}

	return manager
}

// GetAnswer 获取答案（自动fallback到下一个模型）
func (m *ModelManager) GetAnswer(ctx context.Context, question string) (string, error) {
	if len(m.models) == 0 {
		return "", fmt.Errorf("没有可用的模型，请先配置模型API Key")
	}

	var lastErr error
	for _, model := range m.models {
		answer, err := model.GetAnswer(ctx, question)
		if err == nil && answer != "" {
			return answer, nil
		}
		lastErr = err
		// 模型调用失败，尝试下一个
	}

	return "", fmt.Errorf("所有模型都调用失败: %v", lastErr)
}

// HasAvailableModel 检查是否有可用模型
func (m *ModelManager) HasAvailableModel() bool {
	return len(m.models) > 0
}

// GetModelNames 获取可用模型名称列表
func (m *ModelManager) GetModelNames() []string {
	names := make([]string, len(m.models))
	for i, model := range m.models {
		names[i] = model.Name()
	}
	return names
}
