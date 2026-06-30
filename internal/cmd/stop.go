package cmd

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"alvus/internal/config"

	"github.com/spf13/cobra"
)

const pidFileName = "alvus.pid"

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop all running instances",
	Long:  `Stop the alvus proxy server and all its instances gracefully.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Method 1: Try PID file first
		pidData, err := os.ReadFile(pidFileName)
		if err == nil {
			pidStr := strings.TrimSpace(string(pidData))
			pid, err := strconv.Atoi(pidStr)
			if err == nil && pid > 0 {
				fmt.Printf("Stopping alvus process (PID %d)...\n", pid)

				// Cross-platform process termination
				proc, err := os.FindProcess(pid)
				if err == nil {
					if err := proc.Signal(os.Interrupt); err != nil {
						// On Windows, os.Interrupt might not work for non-child processes
						// Fall through to method 2
						fmt.Println("PID signal failed, trying shutdown API...")
					} else {
						// Wait for process to exit
						done := make(chan struct{})
						go func() {
							proc.Wait()
							close(done)
						}()
						select {
						case <-done:
							fmt.Println("Alvus stopped gracefully")
							os.Remove(pidFileName)
							return nil
						case <-time.After(10 * time.Second):
							fmt.Println("Timed out waiting for graceful shutdown")
							// Fall through to method 2 as last resort
						}
					}
				}
			}
		}

		// Method 2: Check running instances via health endpoints
		source, fromToml, derr := config.DetectConfigSource("")
		if derr == nil && fromToml {
			providers, derr := config.LoadAllTomlProviders(source)
			if derr == nil {
				client := &http.Client{Timeout: 3 * time.Second}
				names := make([]string, 0, len(providers))
				for n := range providers {
					names = append(names, n)
				}
				sort.Strings(names)

				for _, name := range names {
					cfg := providers[name]
					healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", cfg.Port)
					resp, err := client.Get(healthURL)
					if err == nil {
						resp.Body.Close()
						fmt.Printf("Instance %q (port %d) is still running\n", name, cfg.Port)
					}
				}
			}
		}

		fmt.Println("Could not stop alvus via PID file. Try:")
		fmt.Println("  - Windows: taskkill /F /PID $(cat alvus.pid)")
		fmt.Println("  - Linux/macOS: kill $(cat alvus.pid)")
		return fmt.Errorf("failed to stop alvus process")
	},
}