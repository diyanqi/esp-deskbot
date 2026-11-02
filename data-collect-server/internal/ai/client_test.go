package ai

import "testing"

func TestParseReportHandlesCodeFenceAndExtraText(t *testing.T) {
	content := "```json\n" + `{
		"score": 72,
		"level": "good",
		"headline": "整体不错",
		"summary": "保持得不错。",
		"findings": [{"title": "提醒", "detail": "注意坐直", "severity": "warn"}],
		"risk_windows": ["周一 09:00-10:00"],
		"suggestions": ["每 40 分钟起身活动"],
		"encouragement": "继续加油",
		"disclaimer": "仅供参考"
	}` + "\n```\n以上是报告。"

	report, err := parseReport(content)
	if err != nil {
		t.Fatalf("parseReport returned error: %v", err)
	}
	if report.Score != 72 || report.Level != "good" {
		t.Fatalf("unexpected score/level: %+v", report)
	}
	if len(report.Findings) != 1 || report.Findings[0].Severity != "warn" {
		t.Fatalf("unexpected findings: %+v", report.Findings)
	}
	if len(report.RiskWindows) != 1 || len(report.Suggestions) != 1 {
		t.Fatalf("unexpected lists: %+v", report)
	}
}

func TestParseReportRejectsNonJSON(t *testing.T) {
	if _, err := parseReport("抱歉，我无法生成报告。"); err == nil {
		t.Fatal("expected an error for non-JSON content")
	}
}

func TestLevelForScore(t *testing.T) {
	cases := map[int]string{
		90: "excellent",
		75: "good",
		60: "attention",
		20: "risk",
	}
	for score, want := range cases {
		if got := LevelForScore(score); got != want {
			t.Fatalf("LevelForScore(%d) = %q, want %q", score, got, want)
		}
	}
}
