package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dashHTML string

var rootCmd = &cobra.Command{
	Use:   "alvus",
	Short: "API Key rotation proxy for AI providers",
	Run: func(cmd *cobra.Command, args []string) {
		local, _ := cmd.Flags().GetBool("local")
		networkOnly, _ := cmd.Flags().GetBool("network-only")
		tag, _ := cmd.Flags().GetString("tag")
		startServer(dashHTML, local, networkOnly, tag)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("alvus version unknown")
	},
}

func Execute(dashboardHTML string) error {
	dashHTML = dashboardHTML
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().Bool("local", false, "Bind to 127.0.0.1 (local access only)")
	rootCmd.PersistentFlags().Bool("network-only", false, "Bind to 0.0.0.0 (accessible via LAN)")
	rootCmd.PersistentFlags().String("tag", "", "Process identity tag (empty = production)")
	rootCmd.AddCommand(versionCmd)
}