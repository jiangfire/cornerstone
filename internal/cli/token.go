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
	Short: "token management",
	Long:  `Manage API tokens. Supports list, create, update, delete subcommands. Requires MASTER_TOKEN env var.`,
}

var tokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "list all tokens",
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
	Short: "create a new token",
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
				return fmt.Errorf("invalid expiration format, use RFC3339: %w", err)
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

		if jsonOutput {
			return printJSON(map[string]interface{}{"id": token.ID, "name": token.Name, "token": token.Token})
		}
		fmt.Println("token created successfully!")
		fmt.Printf("  ID:    %s\n", token.ID)
		fmt.Printf("  Name:  %s\n", token.Name)
		fmt.Printf("  Token: %s\n", token.Token)
		fmt.Println("\nPlease keep this token safe; it will only be shown once.")
		return nil
	},
}

var tokenUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "update a token",
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
				return fmt.Errorf("invalid expiration format, use RFC3339: %w", err)
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
	Short: "delete a token",
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
		fmt.Println("token deleted")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tokenCmd)
	tokenCmd.AddCommand(tokenListCmd)
	tokenCmd.AddCommand(tokenCreateCmd)
	tokenCmd.AddCommand(tokenUpdateCmd)
	tokenCmd.AddCommand(tokenDeleteCmd)

	tokenCreateCmd.Flags().StringP("scopes", "s", "", "scopes (JSON)")
	tokenCreateCmd.Flags().StringP("expires", "e", "", "expiration time (RFC3339)")

	tokenUpdateCmd.Flags().StringP("scopes", "s", "", "scopes (JSON)")
	tokenUpdateCmd.Flags().StringP("expires", "e", "", "expiration time (RFC3339)")
}
