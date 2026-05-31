package cli

import (
	"fmt"
	"time"

	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Token 管理",
	Long:  `管理 API Token。支持 list、create、update、delete 子命令。需要 MASTER_TOKEN 环境变量。`,
}

var tokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有 Token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		masterToken, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewTokenService(db.DB())
		tokens, err := svc.ListTokens(masterToken, true)
		if err != nil {
			return err
		}
		return printJSON(tokens)
	},
}

var tokenCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "创建新 Token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		if _, err := getMasterTokenID(); err != nil {
			return err
		}

		scopes, _ := cmd.Flags().GetString("scopes")
		expires, _ := cmd.Flags().GetString("expires")

		var expiresAt *time.Time
		if expires != "" {
			t, err := time.Parse(time.RFC3339, expires)
			if err != nil {
				return fmt.Errorf("过期时间格式错误，请使用 RFC3339 格式: %w", err)
			}
			expiresAt = &t
		}

		svc := services.NewTokenService(db.DB())
		token, err := svc.CreateToken(services.CreateTokenRequest{
			Name:      args[0],
			Scopes:    scopes,
			ExpiresAt: expiresAt,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Token 创建成功!\n")
		fmt.Printf("  ID:    %s\n", token.ID)
		fmt.Printf("  Name:  %s\n", token.Name)
		fmt.Printf("  Token: %s\n", token.Token)
		fmt.Printf("\n请妥善保管 Token，此值仅显示一次。\n")
		return nil
	},
}

var tokenUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "更新 Token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		if _, err := getMasterTokenID(); err != nil {
			return err
		}

		scopes, _ := cmd.Flags().GetString("scopes")
		expires, _ := cmd.Flags().GetString("expires")

		var expiresAt *time.Time
		if expires != "" {
			t, err := time.Parse(time.RFC3339, expires)
			if err != nil {
				return fmt.Errorf("过期时间格式错误，请使用 RFC3339 格式: %w", err)
			}
			expiresAt = &t
		}

		svc := services.NewTokenService(db.DB())
		token, err := svc.UpdateToken(args[0], scopes, expiresAt)
		if err != nil {
			return err
		}
		return printJSON(token)
	},
}

var tokenDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "删除 Token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		masterToken, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewTokenService(db.DB())
		if err := svc.DeleteToken(masterToken, args[0], true); err != nil {
			return err
		}
		fmt.Println("Token 已删除")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tokenCmd)
	tokenCmd.AddCommand(tokenListCmd)
	tokenCmd.AddCommand(tokenCreateCmd)
	tokenCmd.AddCommand(tokenUpdateCmd)
	tokenCmd.AddCommand(tokenDeleteCmd)

	tokenCreateCmd.Flags().StringP("scopes", "s", "", "权限范围(JSON)")
	tokenCreateCmd.Flags().StringP("expires", "e", "", "过期时间(RFC3339)")

	tokenUpdateCmd.Flags().StringP("scopes", "s", "", "权限范围(JSON)")
	tokenUpdateCmd.Flags().StringP("expires", "e", "", "过期时间(RFC3339)")
}
