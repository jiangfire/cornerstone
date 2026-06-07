package cli

import (
	"fmt"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "cache management",
	Long:  "Manage Cornerstone caches. Supports the clear subcommand.",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "clear all caches",
	Long:  "Clear field cache and token cache.",
	RunE: func(cmd *cobra.Command, args []string) error {
		services.SharedFieldCache.Clear()
		authz.ClearTokenCache()
		fmt.Println("all caches cleared")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheClearCmd)
}
