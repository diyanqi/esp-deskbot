package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"data-collect-server/internal/store"
)

func main() {
	var (
		dsn      = flag.String("dsn", "", "PostgreSQL DSN (defaults to DATABASE_URL)")
		hours    = flag.Int("hours", 24, "hours of history to generate")
		interval = flag.Duration("interval", time.Minute, "sample interval")
		devices  = flag.String("device", "", "comma-separated device IDs (defaults to the two demo devices)")
		reset    = flag.Bool("reset", true, "delete existing data for the selected devices first")
	)
	flag.Parse()

	if *dsn == "" {
		*dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL or -dsn is required")
		os.Exit(1)
	}
	if *hours <= 0 {
		fmt.Fprintln(os.Stderr, "-hours must be positive")
		os.Exit(1)
	}
	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "-interval must be positive")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	database, err := store.New(ctx, *dsn, store.Options{
		MaxConns:       4,
		MinConns:       1,
		ConnectTimeout: 10 * time.Second,
		RunMigrations:  true,
		Logger:         logger,
	})
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	specs := store.DefaultMockDevices()
	if *devices != "" {
		specs = nil
		for index, id := range strings.Split(*devices, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			specs = append(specs, store.MockDeviceSpec{
				DeviceID:       id,
				NodeID:         id,
				ProductID:      "desk-health-demo",
				AppID:          "desk-health-demo",
				GatewayID:      id,
				Firmware:       "0.1.0",
				Seed:           int64(42 + index*37),
				LightBase:      45 + float64(index*7),
				LightAmplitude: 28,
				PostureProfile: []string{"normal", "restless", "sleepy"}[index%3],
			})
		}
	}
	if len(specs) == 0 {
		fmt.Fprintln(os.Stderr, "no devices to seed")
		os.Exit(1)
	}

	to := time.Now().UTC().Truncate(time.Minute)
	from := to.Add(-time.Duration(*hours) * time.Hour)

	result, err := database.SeedMockData(ctx, store.MockSeedOptions{
		Devices:  specs,
		From:     from,
		To:       to,
		Interval: *interval,
		Reset:    *reset,
	})
	if err != nil {
		logger.Error("seed mock data", "error", err)
		os.Exit(1)
	}

	fmt.Printf("seeded %d devices, %d telemetry rows, %d events\n",
		result.Devices, result.Telemetry, result.Events)
	fmt.Printf("range: %s .. %s\n",
		from.Format(time.RFC3339), to.Format(time.RFC3339))
	for _, spec := range specs {
		fmt.Printf("  device: %s (%s)\n", spec.DeviceID, spec.PostureProfile)
	}
}
