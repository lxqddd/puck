package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	defaultGeminiBaseURL    = "https://generativelanguage.googleapis.com"
)

// ListModels 拉取供应商可用的模型列表。kind 取值与 store.ProviderType* 一致。
func ListModels(ctx context.Context, kind, baseURL, apiKey string) ([]string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	client := newHTTPClient()

	switch kind {
	case "anthropic":
		return listAnthropicModels(ctx, client, baseURL, apiKey)
	case "gemini":
		return listGeminiModels(ctx, client, baseURL, apiKey)
	default:
		return listOpenAICompatibleModels(ctx, client, baseURL, apiKey)
	}
}

func listOpenAICompatibleModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]string, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("请填写服务地址")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	body, err := fetchJSON(client, req)
	if err != nil {
		return nil, err
	}

	// OpenAI 使用 data[].id；Ollama 原生接口使用 models[].name。
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析模型列表失败：%w", err)
	}

	ids := make([]string, 0, len(payload.Data)+len(payload.Models))
	for _, item := range payload.Data {
		ids = append(ids, item.ID)
	}
	for _, item := range payload.Models {
		if item.ID != "" {
			ids = append(ids, item.ID)
			continue
		}
		ids = append(ids, item.Name)
	}
	return normalizeModelIDs(ids), nil
}

func listAnthropicModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("请配置 Anthropic API Key")
	}
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}

	ids := make([]string, 0, 32)
	afterID := ""
	for page := 0; page < 10; page++ {
		endpoint := baseURL + "/v1/models?limit=100"
		if afterID != "" {
			endpoint += "&after_id=" + url.QueryEscape(afterID)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		body, err := fetchJSON(client, req)
		if err != nil {
			return nil, err
		}

		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析模型列表失败：%w", err)
		}
		for _, model := range payload.Data {
			ids = append(ids, model.ID)
		}
		if !payload.HasMore || payload.LastID == "" || payload.LastID == afterID {
			break
		}
		afterID = payload.LastID
	}
	return normalizeModelIDs(ids), nil
}

func listGeminiModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("请配置 Gemini API Key")
	}
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}

	ids := make([]string, 0, 32)
	pageToken := ""
	for page := 0; page < 10; page++ {
		query := url.Values{}
		query.Set("pageSize", "1000")
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1beta/models?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-goog-api-key", apiKey)

		body, err := fetchJSON(client, req)
		if err != nil {
			return nil, err
		}

		var payload struct {
			Models []struct {
				Name                       string   `json:"name"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析模型列表失败：%w", err)
		}
		for _, model := range payload.Models {
			name := strings.TrimPrefix(model.Name, "models/")
			if name == "" {
				continue
			}
			if len(model.SupportedGenerationMethods) > 0 && !containsString(model.SupportedGenerationMethods, "generateContent") {
				continue
			}
			ids = append(ids, name)
		}
		if payload.NextPageToken == "" {
			break
		}
		pageToken = payload.NextPageToken
	}
	return normalizeModelIDs(ids), nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 自定义鉴权头不会被 net/http 自动剥离，跨主机重定向必须拒绝，避免泄露 Key。
			if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func fetchJSON(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取模型列表失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, providerHTTPError(resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取模型列表失败：%w", err)
	}
	return body, nil
}

func normalizeModelIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	models := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	sort.Strings(models)
	return models
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
