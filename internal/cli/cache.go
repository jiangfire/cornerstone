package cli

import (
	"fmt"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "缓存管理",
	Long:  "管理 Cornerstone 的缓存，支持 clear 子命令。",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "清空所有缓存",
	Long:  "清空字段缓存和 Token 缓存。",
	RunE: func(cmd *cobra.Command, args []string) error {
		services.SharedFieldCache.Clear()
		authz.ClearTokenCache()
		fmt.Println("所有缓存已清空")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheClearCmd)
}
