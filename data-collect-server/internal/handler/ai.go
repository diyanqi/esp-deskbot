package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"data-collect-server/internal/ai"
	"data-collect-server/internal/store"
)

type aiReportRequest struct {
	DeviceID string `json:"device_id"`
	Hours    int    `json:"hours"`
}

func (h *Handler) handleAIReport(w http.ResponseWriter, r *http.Request) {
	var request aiReportRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "invalid JSON body")
		return
	}
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	if request.DeviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}
	if request.Hours == 0 {
		request.Hours = 24
	}
	if request.Hours < 1 || request.Hours > 168 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "hours must be between 1 and 168")
		return
	}

	to := time.Now().UTC()
	from := to.Add(-time.Duration(request.Hours) * time.Hour)
	insights, err := h.store.GetInsights(r.Context(), request.DeviceID, store.InsightsRange{From: from, To: to})
	if err != nil {
		h.logger.Error("load insights for AI report", "device_id", request.DeviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to aggregate insights")
		return
	}

	aiInput := map[string]any{
		"device_id":    request.DeviceID,
		"hours":        request.Hours,
		"window":       map[string]string{"from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339)},
		"summary":      insights.Summary,
		"risk_windows": insights.RiskWindows,
		"has_data":     insights.HasData,
	}

	var report *ai.Report
	if h.aiClient != nil {
		report, err = h.aiClient.Analyze(r.Context(), aiInput)
		if err != nil {
			h.logger.Warn("AI report failed; using deterministic fallback",
				"device_id", request.DeviceID, "error", err)
			report = nil
		}
	}
	if report == nil {
		report = fallbackReport(insights, request.Hours)
	}
	report.DeviceID = request.DeviceID
	report.Hours = request.Hours
	if report.Level == "" {
		report.Level = ai.LevelForScore(report.Score)
	}
	if report.GeneratedAt.IsZero() {
		report.GeneratedAt = time.Now().UTC()
	}
	if report.Disclaimer == "" {
		report.Disclaimer = "本报告由数据统计生成，仅供参考，不构成医疗建议。"
	}
	if report.Findings == nil {
		report.Findings = []ai.Finding{}
	}
	if report.RiskWindows == nil {
		report.RiskWindows = []string{}
	}
	if report.Suggestions == nil {
		report.Suggestions = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": report})
}

func fallbackReport(insights *store.Insights, hours int) *ai.Report {
	summary := insights.Summary
	score := insights.HealthScore
	level := ai.LevelForScore(score)
	goodPercent := summary.GoodRate * 100
	lightPercent := summary.LightComfort * 100

	headline := "今天坐姿表现很棒，继续保持"
	switch level {
	case "good":
		headline = "整体不错，注意几个小细节"
	case "attention":
		headline = "坐姿习惯还有进步空间"
	case "risk":
		headline = "今天需要更多关注和陪伴"
	}

	overview := fmt.Sprintf(
		"过去 %d 小时，坐姿良好率 %.0f%%，共出现 %d 次坐姿提醒，最长一次持续 %d 分钟；平均桌面光照 %.0f%%，光照舒适度 %.0f%%。",
		hours, goodPercent, summary.AlertEvents, summary.LongestBadStreakMinutes, summary.AvgLight, lightPercent,
	)
	if !insights.HasData {
		overview = "当前时间段还没有足够的监测数据。请确认设备已在线，并让孩子在书桌前保持几分钟，系统会自动生成分析。"
	}

	findings := []ai.Finding{
		{
			Title:    "坐姿良好率",
			Detail:   fmt.Sprintf("统计窗口内 %.0f%% 的时间保持良好坐姿。", goodPercent),
			Severity: severityForRate(summary.GoodRate),
		},
		{
			Title:    "最长连续提醒",
			Detail:   fmt.Sprintf("最长一次不良坐姿持续约 %d 分钟。", summary.LongestBadStreakMinutes),
			Severity: severityForStreak(summary.LongestBadStreakMinutes),
		},
		{
			Title:    "桌面光照",
			Detail:   fmt.Sprintf("平均光照 %.0f，舒适度 %.0f%%。", summary.AvgLight, lightPercent),
			Severity: severityForLight(summary.AvgLight),
		},
	}

	riskWindows := make([]string, 0, len(insights.RiskWindows))
	for _, window := range insights.RiskWindows {
		riskWindows = append(riskWindows, fmt.Sprintf("%s %02d:00-%02d:00（告警率 %.0f%%）",
			weekdayName(window.Weekday), window.Hour, (window.Hour+1)%24, window.AlertRate*100))
	}
	if len(riskWindows) == 0 {
		riskWindows = append(riskWindows, "暂未发现明显的高风险时段")
	}

	suggestions := []string{
		"每 40 分钟提醒孩子起身活动 2-3 分钟，顺便远眺放松眼睛。",
		"把屏幕顶端调整到接近眼睛高度，双脚平放地面，腰背自然挺直。",
		"多用具体表扬代替批评，例如“刚才坐得很直，保持得很好”。",
	}
	if summary.AvgLight < 40 {
		suggestions = append(suggestions, "桌面光照偏低，建议补充一盏无频闪台灯，避免孩子在暗光下弯腰凑近。")
	}
	if summary.LongestBadStreakMinutes >= 20 {
		suggestions = append(suggestions, "连续提醒时段可以和孩子一起检查桌椅高度，必要时加坐垫或脚踏。")
	}
	if summary.AlertEvents == 0 && insights.HasData {
		suggestions = append(suggestions, "今天没有明显告警，可以和孩子约定一个小奖励，强化好习惯。")
	}

	return &ai.Report{
		Score:         score,
		Level:         level,
		Headline:      headline,
		Summary:       overview,
		Findings:      findings,
		RiskWindows:   riskWindows,
		Suggestions:   suggestions,
		Encouragement: "每一次挺直腰背，都是在为成长加分。",
		Disclaimer:    "本报告由本地统计数据生成，仅供参考，不构成医疗建议。",
		Source:        "local",
		Model:         "rule-based",
		GeneratedAt:   time.Now().UTC(),
	}
}

func severityForRate(rate float64) string {
	switch {
	case rate >= 0.8:
		return "info"
	case rate >= 0.6:
		return "warn"
	default:
		return "alert"
	}
}

func severityForStreak(minutes int) string {
	switch {
	case minutes <= 5:
		return "info"
	case minutes <= 15:
		return "warn"
	default:
		return "alert"
	}
}

func severityForLight(light float64) string {
	if light < 30 || light > 120 {
		return "warn"
	}
	return "info"
}

func weekdayName(weekday int) string {
	names := [...]string{"", "周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	if weekday < 1 || weekday >= len(names) {
		return "未知"
	}
	return names[weekday]
}
