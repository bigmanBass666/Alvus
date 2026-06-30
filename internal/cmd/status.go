package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"alvus/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show runtime status of all instances",
	Long:  `Query all running instances and display their health, key counts, and request statistics.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Detect config source
		source, fromToml, err := config.DetectConfigSource("")
		if err != nil {
			return fmt.Errorf("failed to detect config source: %w", err)
		}
		if !fromToml {
			return fmt.Errorf("no TOML configuration found (expected at %s)", source)
		}
		if _, statErr := os.Stat(source); statErr != nil {
			return fmt.Errorf("no configuration file found at %s", source)
		}

		// Load all providers
		providers, err := config.LoadAllTomlProviders(source)
		if err != nil {
			return fmt.Errorf("failed to load providers: %w", err)
		}
		if len(providers) == 0 {
			fmt.Println("No providers configured")
			return nil
		}

		client := &http.Client{Timeout: 3 * time.Second}

		// Sort names for deterministic output
		names := make([]string, 0, len(providers))
		for n := range providers {
			names = append(names, n)
		}
		sort.Strings(names)

		fmt.Printf("%-15s %-6s %-10s %-12s %-10s %s\n", "PROVIDER", "PORT", "HEALTH", "ACTIVE KEYS", "REQUESTS", "UPTIME")
		fmt.Println("-----------------------------------------------------------------------------")

		for _, name := range names {
			cfg := providers[name]
			port := cfg.Port
			health := "unknown"
			activeKeys := "?"
			requests := "?"
			uptime := "?"

			// Query health endpoint
			healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
			healthResp, err := client.Get(healthURL)
			if err == nil {
				healthResp.Body.Close()
				health = "UP"
				if healthResp.StatusCode == http.StatusOK {
					health = "UP"
				} else {
					health = fmt.Sprintf("ERR-%d", healthResp.StatusCode)
				}
			} else {
				health = "DOWN"
			}

			// Query stats endpoint
			if health == "UP" {
				statsURL := fmt.Sprintf("http://127.0.0.1:%d/api/stats", port)
				statsResp, err := client.Get(statsURL)
				if err == nil {
					var stats map[string]interface{}
					body, _ := io.ReadAll(statsResp.Body)
					json.Unmarshal(body, &stats)
					statsResp.Body.Close()

					if v, ok := stats["active_keys"]; ok {
						activeKeys = fmt.Sprintf("%v", v)
					}
					if v, ok := stats["total_requests"]; ok {
						requests = fmt.Sprintf("%v", v)
					}
					if v, ok := stats["uptime_seconds"]; ok {
						secs := v.(float64)
						uptime = fmt.Sprintf("%.0fs", secs)
					}
				}
			}

			fmt.Printf("%-15s %-6d %-10s %-12s %-10s %s\n", name, port, health, activeKeys, requests, uptime)
		}

		return nil
	},
}