package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jiangfire/cornerstone/internal/config"
	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	applog "github.com/jiangfire/cornerstone/pkg/log"
	"github.com/spf13/cobra"
)

func ensureDB() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := applog.InitLogger(cfg.Logger); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	if err := retryOperation(func() error {
		return appdb.InitDB(cfg.Database)
	}, 3, time.Second); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	if err := appdb.Migrate(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}

func getMasterTokenID() (string, error) {
	if tokenOverride != "" {
		return tokenOverride, nil
	}
	masterToken := os.Getenv("MASTER_TOKEN")
	if masterToken == "" {
		return "", fmt.Errorf("set MASTER_TOKEN env var or use --token flag")
	}
	return masterToken, nil
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "database management",
	Long:  `Manage Cornerstone database resources. Supports list, create, get, update, delete subcommands.`,
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "list all databases",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		databases, err := svc.ListDatabases(token)
		if err != nil {
			return err
		}
		return printJSON(databases)
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "create a database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		desc, _ := cmd.Flags().GetString("description")
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.CreateDatabase(services.CreateDBRequest{
			Name:        args[0],
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(database)
	},
}

var dbGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "get database details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.GetDatabase(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(database)
	},
}

var dbUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "update a database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.UpdateDatabase(args[0], services.UpdateDBRequest{
			Name:        name,
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(database)
	},
}

var dbDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "delete a database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		if err := svc.DeleteDatabase(args[0], token); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]interface{}{"id": args[0], "deleted": true})
		}
		fmt.Println("database deleted")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbGetCmd)
	dbCmd.AddCommand(dbUpdateCmd)
	dbCmd.AddCommand(dbDeleteCmd)

	dbCreateCmd.Flags().StringP("description", "d", "", "database description")
	dbUpdateCmd.Flags().StringP("name", "n", "", "new name")
	dbUpdateCmd.Flags().StringP("description", "d", "", "new description")
}
