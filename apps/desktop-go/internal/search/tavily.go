package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultEndpoint = "https://api.tavily.com/search"

type Result struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	Content   string `json:"content"`
	Published string `json:"published,omitempty"`
}

type Client struct {
	APIKey   string
	Endpoint string
	HTTP     *http.Client

	mu            sync.Mutex
	cooldownUntil time.Time
}

func NewClient() *Client {
	return &Client{
		APIKey:   strings.TrimSpace(os.Getenv("TAVILY_API_KEY")),
		Endpoint: defaultEndpoint,
		HTTP:     &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *Client) Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("搜索词为空")
	}
	if remaining := c.cooldownRemaining(); remaining > 0 {
		return nil, fmt.Errorf("联网搜索暂时不可用（限流冷却中，约 %d 秒后重试）", int(remaining.Seconds())+1)
	}
	if maxResults <= 0 || maxResults > 8 {
		maxResults = 5
	}
	endpoint := strings.TrimSpace(c.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}

	payload, err := json.Marshal(map[string]any{
		"query":               query,
		"search_depth":        "basic",
		"chunks_per_source":   3,
		"max_results":         maxResults,
		"include_answer":      false,
		"include_raw_content": false,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "puck/0.1")
	if key := strings.TrimSpace(c.APIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	} else {
		req.Header.Set("X-Tavily-Access-Mode", "keyless")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("联网搜索失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取搜索结果失败：%w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case 429:
			c.setCooldown(retryAfter(resp.Header.Get("Retry-After")))
			return nil, fmt.Errorf("联网搜索被限流（HTTP 429），请稍后再试")
		case 401, 403:
			return nil, fmt.Errorf("联网搜索鉴权失败（HTTP %d）", resp.StatusCode)
		default:
			return nil, fmt.Errorf("联网搜索失败（HTTP %d）：%s", resp.StatusCode, truncate(string(body), 300))
		}
	}
	c.clearCooldown()

	var decoded struct {
		Results []struct {
			Title         string `json:"title"`
			URL           string `json:"url"`
			Content       string `json:"content"`
			PublishedDate string `json:"published_date"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败：%w", err)
	}
	results := make([]Result, 0, len(decoded.Results))
	for _, item := range decoded.Results {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		results = append(results, Result{
			Title:     strings.TrimSpace(item.Title),
			URL:       strings.TrimSpace(item.URL),
			Content:   strings.TrimSpace(item.Content),
			Published: strings.TrimSpace(item.PublishedDate),
		})
	}
	return results, nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 || seconds > 600 {
		return 60 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func (c *Client) cooldownRemaining() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Until(c.cooldownUntil)
}

func (c *Client) setCooldown(duration time.Duration) {
	c.mu.Lock()
	c.cooldownUntil = time.Now().Add(duration)
	c.mu.Unlock()
}

func (c *Client) clearCooldown() {
	c.mu.Lock()
	c.cooldownUntil = time.Time{}
	c.mu.Unlock()
}
