package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// InsightsRange is the analysis window.
type InsightsRange struct {
	From time.Time
	To   time.Time
}

// Insights is the aggregated view used by the dashboard and AI report.
type Insights struct {
	DeviceID    string               `json:"device_id"`
	From        time.Time            `json:"from"`
	To          time.Time            `json:"to"`
	Hours       int                  `json:"hours"`
	HasData     bool                 `json:"has_data"`
	HealthScore int                  `json:"health_score"`
	Summary     InsightsSummary      `json:"summary"`
	Hourly      []InsightsHourly     `json:"hourly"`
	Heatmap     []InsightsHeatmap    `json:"heatmap"`
	Timeline    []InsightsTimeline   `json:"timeline"`
	RiskWindows []InsightsRiskWindow `json:"risk_windows"`
}

// InsightsSummary contains headline statistics for the selected window.
type InsightsSummary struct {
	PostureSamples          int64   `json:"posture_samples"`
	Good                    int64   `json:"good"`
	Warning                 int64   `json:"warning"`
	Bad                     int64   `json:"bad"`
	Unknown                 int64   `json:"unknown"`
	GoodRate                float64 `json:"good_rate"`
	AlertRate               float64 `json:"alert_rate"`
	PersonRate              float64 `json:"person_rate"`
	AlertMinutes            int64   `json:"alert_minutes"`
	AlertEvents             int64   `json:"alert_events"`
	SoundEvents             int64   `json:"sound_events"`
	AvgLight                float64 `json:"avg_light"`
	MinLight                float64 `json:"min_light"`
	MaxLight                float64 `json:"max_light"`
	LightComfort            float64 `json:"light_comfort"`
	AvgRSSI                 float64 `json:"avg_rssi"`
	AvgHeapKB               float64 `json:"avg_heap_kb"`
	LongestBadStreakMinutes int     `json:"longest_bad_streak_minutes"`
}

// InsightsHourly is one hour of the selected window.
type InsightsHourly struct {
	Bucket    time.Time `json:"bucket"`
	GoodRate  float64   `json:"good_rate"`
	AlertRate float64   `json:"alert_rate"`
	AvgLight  float64   `json:"avg_light"`
	Samples   int64     `json:"samples"`
}

// InsightsHeatmap is one weekday/hour cell over the last seven days.
type InsightsHeatmap struct {
	Weekday   int     `json:"weekday"` // 1=Monday, 7=Sunday
	Hour      int     `json:"hour"`
	GoodRate  float64 `json:"good_rate"`
	AlertRate float64 `json:"alert_rate"`
	AvgLight  float64 `json:"avg_light"`
	Samples   int64   `json:"samples"`
}

// InsightsTimeline is a five-minute replay point.
type InsightsTimeline struct {
	Time   time.Time `json:"time"`
	Status string    `json:"status"`
	Alert  bool      `json:"alert"`
	Light  float64   `json:"light"`
	Sound  bool      `json:"sound"`
	Person bool      `json:"person"`
}

// InsightsRiskWindow is a recurring high-risk weekday/hour cell.
type InsightsRiskWindow struct {
	Weekday   int     `json:"weekday"`
	Hour      int     `json:"hour"`
	GoodRate  float64 `json:"good_rate"`
	AlertRate float64 `json:"alert_rate"`
	AvgLight  float64 `json:"avg_light"`
	Samples   int64   `json:"samples"`
}

// GetInsights aggregates the selected window and the last seven days.
func (s *Store) GetInsights(ctx context.Context, deviceID string, window InsightsRange) (*Insights, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	if !window.From.Before(window.To) {
		return nil, fmt.Errorf("from must be before to")
	}
	hours := int(math.Round(window.To.Sub(window.From).Hours()))
	heatmapFrom := window.To.Add(-7 * 24 * time.Hour)

	result := &Insights{
		DeviceID: deviceID,
		From:     window.From.UTC(),
		To:       window.To.UTC(),
		Hours:    hours,
	}

	var good, warning, bad, unknownValue int64
	var personCount, personSamples int64
	var alertMinutes, soundEvents int64
	var avgLight, minLight, maxLight, avgRSSI, avgHeap pgtype.Float8

	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'postureStatus'),
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'postureStatus'
				AND value_json #>> '{}' = 'good'),
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'postureStatus'
				AND value_json #>> '{}' = 'warning'),
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'postureStatus'
				AND value_json #>> '{}' = 'bad'),
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'personDetected'
				AND value_num = 1),
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'personDetected'),
			sum(value_num) FILTER (WHERE service_id = 'Posture' AND property = 'alert'),
			sum(value_num) FILTER (WHERE service_id = 'Environment' AND property = 'sound'),
			avg(value_num) FILTER (WHERE service_id = 'Environment' AND property = 'light'),
			min(value_num) FILTER (WHERE service_id = 'Environment' AND property = 'light'),
			max(value_num) FILTER (WHERE service_id = 'Environment' AND property = 'light'),
			avg(value_num) FILTER (WHERE service_id = 'DeviceStatus' AND property = 'rssi'),
			avg(value_num) FILTER (WHERE service_id = 'DeviceStatus' AND property = 'freeHeap')
		FROM telemetry
		WHERE device_id = $1 AND event_time >= $2 AND event_time < $3`,
		deviceID, window.From, window.To,
	).Scan(
		&result.Summary.PostureSamples,
		&good, &warning, &bad,
		&personCount, &personSamples,
		&alertMinutes, &soundEvents,
		&avgLight, &minLight, &maxLight, &avgRSSI, &avgHeap,
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate insights summary: %w", err)
	}

	result.Summary.Good = good
	result.Summary.Warning = warning
	result.Summary.Bad = bad
	unknownValue = result.Summary.PostureSamples - good - warning - bad
	if unknownValue < 0 {
		unknownValue = 0
	}
	result.Summary.Unknown = unknownValue
	result.Summary.AlertMinutes = alertMinutes
	result.Summary.SoundEvents = soundEvents
	result.Summary.AvgLight = floatValue(avgLight)
	result.Summary.MinLight = floatValue(minLight)
	result.Summary.MaxLight = floatValue(maxLight)
	result.Summary.AvgRSSI = floatValue(avgRSSI)
	result.Summary.AvgHeapKB = floatValue(avgHeap) / 1024
	if result.Summary.PostureSamples > 0 {
		result.Summary.GoodRate = float64(good) / float64(result.Summary.PostureSamples)
		result.Summary.AlertRate = float64(alertMinutes) / float64(result.Summary.PostureSamples)
		result.Summary.PersonRate = float64(personCount) / float64(personSamples)
	}
	result.Summary.LightComfort = lightComfort(result.Summary.AvgLight)
	result.HasData = result.Summary.PostureSamples > 0 || avgLight.Valid
	result.HealthScore = healthScore(result.Summary)

	hourly, err := s.queryHourly(ctx, deviceID, window)
	if err != nil {
		return nil, err
	}
	result.Hourly = hourly

	heatmap, err := s.queryHeatmap(ctx, deviceID, heatmapFrom, window.To)
	if err != nil {
		return nil, err
	}
	result.Heatmap = heatmap

	timeline, err := s.queryTimeline(ctx, deviceID, window)
	if err != nil {
		return nil, err
	}
	result.Timeline = timeline

	var alertEvents int64
	longestBad := 0
	currentBad := 0
	previousAlert := false
	for _, point := range timeline {
		if point.Alert && !previousAlert {
			alertEvents++
		}
		previousAlert = point.Alert
		if point.Status == "bad" {
			currentBad++
			if currentBad > longestBad {
				longestBad = currentBad
			}
		} else {
			currentBad = 0
		}
	}
	result.Summary.AlertEvents = alertEvents
	result.Summary.LongestBadStreakMinutes = longestBad * 5

	result.RiskWindows = topRiskWindows(heatmap, 4)
	return result, nil
}

func (s *Store) queryHourly(ctx context.Context, deviceID string, window InsightsRange) ([]InsightsHourly, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			date_trunc('hour', event_time) AS bucket,
			avg(CASE WHEN service_id = 'Posture' AND property = 'postureStatus'
				THEN CASE WHEN value_json #>> '{}' = 'good' THEN 1 ELSE 0 END END) AS good_rate,
			avg(CASE WHEN service_id = 'Posture' AND property = 'alert' THEN value_num END) AS alert_rate,
			avg(CASE WHEN service_id = 'Environment' AND property = 'light' THEN value_num END) AS avg_light,
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'postureStatus') AS samples
		FROM telemetry
		WHERE device_id = $1 AND event_time >= $2 AND event_time < $3
		GROUP BY bucket ORDER BY bucket`,
		deviceID, window.From, window.To)
	if err != nil {
		return nil, fmt.Errorf("query hourly insights: %w", err)
	}
	defer rows.Close()

	items := make([]InsightsHourly, 0, 24)
	for rows.Next() {
		var item InsightsHourly
		var goodRate, alertRate, avgLight pgtype.Float8
		if err := rows.Scan(&item.Bucket, &goodRate, &alertRate, &avgLight, &item.Samples); err != nil {
			return nil, fmt.Errorf("scan hourly insights: %w", err)
		}
		item.Bucket = item.Bucket.UTC()
		item.GoodRate = floatValue(goodRate)
		item.AlertRate = floatValue(alertRate)
		item.AvgLight = floatValue(avgLight)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) queryHeatmap(ctx context.Context, deviceID string, from, to time.Time) ([]InsightsHeatmap, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			CAST(CASE WHEN EXTRACT(DOW FROM event_time) = 0 THEN 7
				ELSE EXTRACT(DOW FROM event_time) END AS int) AS weekday,
			CAST(EXTRACT(HOUR FROM event_time) AS int) AS hour,
			avg(CASE WHEN service_id = 'Posture' AND property = 'postureStatus'
				THEN CASE WHEN value_json #>> '{}' = 'good' THEN 1 ELSE 0 END END) AS good_rate,
			avg(CASE WHEN service_id = 'Posture' AND property = 'alert' THEN value_num END) AS alert_rate,
			avg(CASE WHEN service_id = 'Environment' AND property = 'light' THEN value_num END) AS avg_light,
			count(*) FILTER (WHERE service_id = 'Posture' AND property = 'postureStatus') AS samples
		FROM telemetry
		WHERE device_id = $1 AND event_time >= $2 AND event_time < $3
		GROUP BY weekday, hour
		ORDER BY weekday, hour`,
		deviceID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query heatmap insights: %w", err)
	}
	defer rows.Close()

	items := make([]InsightsHeatmap, 0, 168)
	for rows.Next() {
		var item InsightsHeatmap
		var goodRate, alertRate, avgLight pgtype.Float8
		if err := rows.Scan(&item.Weekday, &item.Hour, &goodRate, &alertRate, &avgLight,
			&item.Samples); err != nil {
			return nil, fmt.Errorf("scan heatmap insights: %w", err)
		}
		item.GoodRate = floatValue(goodRate)
		item.AlertRate = floatValue(alertRate)
		item.AvgLight = floatValue(avgLight)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) queryTimeline(ctx context.Context, deviceID string, window InsightsRange) ([]InsightsTimeline, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			to_timestamp(floor(extract(epoch FROM event_time) / 300) * 300) AS bucket,
			avg(CASE WHEN service_id = 'Posture' AND property = 'alert' THEN value_num END) AS alert_rate,
			avg(CASE WHEN service_id = 'Environment' AND property = 'light' THEN value_num END) AS light,
			avg(CASE WHEN service_id = 'Environment' AND property = 'sound' THEN value_num END) AS sound,
			avg(CASE WHEN service_id = 'Posture' AND property = 'personDetected' THEN value_num END) AS person,
			avg(CASE WHEN service_id = 'Posture' AND property = 'postureStatus'
				THEN CASE WHEN value_json #>> '{}' = 'good' THEN 1 ELSE 0 END END) AS good_rate,
			avg(CASE WHEN service_id = 'Posture' AND property = 'postureStatus'
				THEN CASE WHEN value_json #>> '{}' = 'warning' THEN 1 ELSE 0 END END) AS warning_rate,
			avg(CASE WHEN service_id = 'Posture' AND property = 'postureStatus'
				THEN CASE WHEN value_json #>> '{}' = 'bad' THEN 1 ELSE 0 END END) AS bad_rate
		FROM telemetry
		WHERE device_id = $1 AND event_time >= $2 AND event_time < $3
		GROUP BY bucket ORDER BY bucket`,
		deviceID, window.From, window.To)
	if err != nil {
		return nil, fmt.Errorf("query insights timeline: %w", err)
	}
	defer rows.Close()

	items := make([]InsightsTimeline, 0, 288)
	for rows.Next() {
		var point InsightsTimeline
		var alertRate, light, sound, person, goodRate, warningRate, badRate pgtype.Float8
		if err := rows.Scan(&point.Time, &alertRate, &light, &sound, &person,
			&goodRate, &warningRate, &badRate); err != nil {
			return nil, fmt.Errorf("scan insights timeline: %w", err)
		}
		point.Time = point.Time.UTC()
		point.Alert = floatValue(alertRate) > 0.5
		point.Light = floatValue(light)
		point.Sound = floatValue(sound) > 0.5
		point.Person = floatValue(person) > 0.5
		switch {
		case floatValue(badRate) > 0.5:
			point.Status = "bad"
		case floatValue(warningRate) > 0.5:
			point.Status = "warning"
		case floatValue(goodRate) > 0.5:
			point.Status = "good"
		default:
			point.Status = "unknown"
		}
		items = append(items, point)
	}
	return items, rows.Err()
}

func topRiskWindows(cells []InsightsHeatmap, limit int) []InsightsRiskWindow {
	candidates := make([]InsightsHeatmap, 0, len(cells))
	for _, cell := range cells {
		if cell.Samples > 0 && cell.AlertRate > 0.02 {
			candidates = append(candidates, cell)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].AlertRate == candidates[j].AlertRate {
			return candidates[i].GoodRate < candidates[j].GoodRate
		}
		return candidates[i].AlertRate > candidates[j].AlertRate
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result := make([]InsightsRiskWindow, 0, len(candidates))
	for _, cell := range candidates {
		result = append(result, InsightsRiskWindow{
			Weekday:   cell.Weekday,
			Hour:      cell.Hour,
			GoodRate:  cell.GoodRate,
			AlertRate: cell.AlertRate,
			AvgLight:  cell.AvgLight,
			Samples:   cell.Samples,
		})
	}
	return result
}

func floatValue(value pgtype.Float8) float64 {
	if !value.Valid {
		return 0
	}
	return value.Float64
}

func lightComfort(avgLight float64) float64 {
	if avgLight <= 0 {
		return 0
	}
	comfort := 1 - math.Abs(avgLight-60)/60
	if comfort < 0 {
		return 0
	}
	if comfort > 1 {
		return 1
	}
	return comfort
}

func healthScore(summary InsightsSummary) int {
	if summary.PostureSamples == 0 {
		return 0
	}
	score := summary.GoodRate*60 +
		(1-summary.AlertRate)*20 +
		summary.LightComfort*10 +
		summary.PersonRate*10
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return int(math.Round(score))
}
