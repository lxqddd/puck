package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DeepSeekClient 调用 DeepSeek 的 Search / Fetch API，与对话共用同一把 API Key。
type DeepSeekClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client

	mu            sync.Mutex
	cooldownUntil time.Time
}

func NewDeepSeekClient(baseURL, apiKey string) *DeepSeekClient {
	return &DeepSeekClient{
		BaseURL: baseURL,
		APIKey:  strings.TrimSpace(apiKey),
		HTTP:    &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *DeepSeekClient) Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("搜索词为空")
	}
	if remaining := c.cooldownRemaining(); remaining > 0 {
		return nil, fmt.Errorf("DeepSeek 搜索限流冷却中，约 %d 秒后重试", int(remaining.Seconds())+1)
	}
	runes := []rune(query)
	if len(runes) > 128 {
		query = string(runes[:128])
	}
	if maxResults <= 0 || maxResults > 50 {
		maxResults = 5
	}

	body, err := c.post(ctx, "/tools/search", map[string]any{
		"query":        query,
		"count":        maxResults,
		"need_content": false,
	})
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
			Summary string `json:"summary"`
			Content string `json:"content"`
			Date    string `json:"date"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("解析 DeepSeek 搜索结果失败：%w", err)
	}
	results := make([]Result, 0, len(decoded.Results))
	for _, item := range decoded.Results {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		content := strings.TrimSpace(item.Summary)
		if content == "" {
			content = strings.TrimSpace(item.Snippet)
		}
		if content == "" {
			content = strings.TrimSpace(item.Content)
		}
		results = append(results, Result{
			Title:     strings.TrimSpace(item.Title),
			URL:       strings.TrimSpace(item.URL),
			Content:   content,
			Published: strings.TrimSpace(item.Date),
		})
	}
	return results, nil
}

func (c *DeepSeekClient) Fetch(ctx context.Context, pageURL string) (string, string, error) {
	pageURL = strings.TrimSpace(pageURL)
	parsed, err := url.Parse(pageURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", "", fmt.Errorf("网页地址无效：需要 HTTP/HTTPS URL")
	}
	if parsed.User != nil {
		return "", "", fmt.Errorf("网页地址不能包含用户名或密码")
	}
	if len(pageURL) > 8192 {
		return "", "", fmt.Errorf("网页地址过长")
	}
	body, err := c.post(ctx, "/tools/fetch", map[string]any{"url": pageURL})
	if err != nil {
		return "", "", err
	}
	var decoded struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", "", fmt.Errorf("解析 DeepSeek 抓取结果失败：%w", err)
	}
	return strings.TrimSpace(decoded.Title), strings.TrimSpace(decoded.Content), nil
}

func (c *DeepSeekClient) post(ctx context.Context, suffix string, payload any) ([]byte, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("缺少 DeepSeek API Key")
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := deepSeekEndpoint(c.BaseURL, suffix)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("User-Agent", "puck/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek 联网请求失败：%w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 DeepSeek 响应失败：%w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 429 {
			c.setCooldown(retryAfter(resp.Header.Get("Retry-After")))
		}
		return nil, deepSeekError(resp.StatusCode, string(responseBody))
	}
	c.clearCooldown()
	return responseBody, nil
}

func deepSeekEndpoint(baseURL, suffix string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.deepseek.com"
	}
	if strings.HasSuffix(base, "/v1") {
		return base + suffix
	}
	return base + "/v1" + suffix
}

func deepSeekError(status int, body string) error {
	detail := truncate(strings.TrimSpace(body), 300)
	switch status {
	case 400:
		return fmt.Errorf("DeepSeek 搜索请求不合法（HTTP 400）：%s", detail)
	case 401:
		return fmt.Errorf("DeepSeek API Key 无效（HTTP 401）")
	case 402:
		return fmt.Errorf("DeepSeek 账户余额不足（HTTP 402）")
	case 403:
		return fmt.Errorf("DeepSeek 拒绝抓取该网页（HTTP 403）")
	case 404:
		return fmt.Errorf("DeepSeek 未能获取该网页内容（HTTP 404）")
	case 408, 500, 504:
		return fmt.Errorf("DeepSeek 联网服务暂时不可用（HTTP %d）", status)
	case 429:
		return fmt.Errorf("DeepSeek 联网请求过于频繁（HTTP 429）")
	default:
		return fmt.Errorf("DeepSeek 联网请求失败（HTTP %d）：%s", status, detail)
	}
}

func (c *DeepSeekClient) cooldownRemaining() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Until(c.cooldownUntil)
}

func (c *DeepSeekClient) setCooldown(duration time.Duration) {
	c.mu.Lock()
	c.cooldownUntil = time.Now().Add(duration)
	c.mu.Unlock()
}

func (c *DeepSeekClient) clearCooldown() {
	c.mu.Lock()
	c.cooldownUntil = time.Time{}
	c.mu.Unlock()
}
