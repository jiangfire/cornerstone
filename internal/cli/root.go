package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "cornerstone",
	Short: "Cornerstone - 轻量数据资产平台 CLI",
	Long: `Cornerstone 是一个轻量数据资产平台，面向测试、开发和内部数据管理场景。
核心定位："数据库 + Token 接口 + Query DSL + AI 助手 + MCP 协议"。

通过 CLI 可以直接管理数据资产，也可以启动 HTTP API + MCP 服务器供 AI Agent 调用。`,
	Version: Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("Cornerstone %s\n", Version))
}
