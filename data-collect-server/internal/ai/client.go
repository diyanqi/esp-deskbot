// Package ai calls an OpenAI-compatible chat completion endpoint to produce a
// parent-facing posture health report.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Finding is one highlighted observation in the report.
type Finding struct {
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
}

// Report is the structured result rendered by the dashboard.
type Report struct {
	DeviceID      string    `json:"device_id"`
	Hours         int       `json:"hours"`
	Score         int       `json:"score"`
	Level         string    `json:"level"`
	Headline      string    `json:"headline"`
	Summary       string    `json:"summary"`
	Findings      []Finding `json:"findings"`
	RiskWindows   []string  `json:"risk_windows"`
	Suggestions   []string  `json:"suggestions"`
	Encouragement string    `json:"encouragement"`
	Disclaimer    string    `json:"disclaimer"`
	Source        string    `json:"source"`
	Model         string    `json:"model"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// Client calls an OpenAI-compatible chat completions API.
type Client struct {
	endpoint string
	model    string
	apiKey   string
	http     *http.Client
}

// New creates a client. A nil client is safe to use and returns an error.
func New(endpoint, model, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return &Client{
		endpoint: strings.TrimSpace(endpoint),
		model:    strings.TrimSpace(model),
		apiKey:   strings.TrimSpace(apiKey),
		http:     &http.Client{Timeout: timeout},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const systemPrompt = `你是一位面向家长的儿童坐姿健康顾问。请根据提供的监测统计数据，用温和、具体、可执行的中文给出建议。
要求：
1. 不做医学诊断，不制造恐慌，不承诺治疗效果。
2. 语言适合家长阅读，先肯定孩子的努力，再给出改进建议。
3. 只输出一个 JSON 对象，不要 Markdown 代码块，不要额外解释。
JSON 结构：
{
  "score": 0 到 100 的整数,
  "level": "excellent" | "good" | "attention" | "risk",
  "headline": "一句话总结",
  "summary": "2-3 句总体评价",
  "findings": [{"title": "发现", "detail": "说明", "severity": "info" | "warn" | "alert"}],
  "risk_windows": ["周一 09:00-10:00", "..."],
  "suggestions": ["给家长的具体建议", "..."],
  "encouragement": "给孩子的一句鼓励",
  "disclaimer": "本报告由数据统计和 AI 生成，仅供参考，不构成医疗建议。"
}`

// Analyze sends the summary to the model and parses the JSON report.
func (c *Client) Analyze(ctx context.Context, input any) (*Report, error) {
	if c == nil || c.endpoint == "" || c.model == "" {
		return nil, errors.New("AI client is not configured")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode AI input: %w", err)
	}

	body, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "监测数据（JSON）：\n" + string(payload)},
		},
		Temperature: 0.4,
	})
	if err != nil {
		return nil, fmt.Errorf("encode AI request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create AI request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call AI endpoint: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("AI endpoint returned %d: %s", response.StatusCode, truncate(string(responseBody), 240))
	}

	var completion chatResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, errors.New("AI response has no choices")
	}

	report, err := parseReport(completion.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	report.Source = "ai"
	report.Model = c.model
	report.GeneratedAt = time.Now().UTC()
	if report.Level == "" {
		report.Level = LevelForScore(report.Score)
	}
	return report, nil
}

func parseReport(content string) (*Report, error) {
	text := strings.TrimSpace(content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return nil, errors.New("AI response is not a JSON object")
	}
	text = text[start : end+1]

	var report Report
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		return nil, fmt.Errorf("decode AI report JSON: %w", err)
	}
	if report.Score < 0 {
		report.Score = 0
	}
	if report.Score > 100 {
		report.Score = 100
	}
	if report.Findings == nil {
		report.Findings = []Finding{}
	}
	if report.RiskWindows == nil {
		report.RiskWindows = []string{}
	}
	if report.Suggestions == nil {
		report.Suggestions = []string{}
	}
	return &report, nil
}

// LevelForScore maps a numeric score to a stable UI level.
func LevelForScore(score int) string {
	switch {
	case score >= 85:
		return "excellent"
	case score >= 70:
		return "good"
	case score >= 55:
		return "attention"
	default:
		return "risk"
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
