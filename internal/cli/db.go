package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/config"
	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/dto"
	"github.com/jiangfire/cornerstone/pkg/db"
	applog "github.com/jiangfire/cornerstone/pkg/log"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func ensureDB() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// In CLI mode (non-serve), suppress log output to keep stdout clean for results only.
	if !jsonOutput {
		cfg.Logger.Level = "fatal"
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

func currentDB() (*gorm.DB, error) {
	if !db.IsInitialized() {
		return nil, fmt.Errorf("database not initialized")
	}
	return db.DB(), nil
}

func getAuthTokenID() (string, error) {
	credential, err := getMasterTokenID()
	if err != nil {
		return "", err
	}

	if masterToken := os.Getenv("MASTER_TOKEN"); masterToken != "" && credential == masterToken {
		return credential, nil
	}

	conn, err := currentDB()
	if err != nil {
		return "", err
	}

	token, err := authz.FindTokenByValue(conn, credential)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return credential, nil
		}
		return "", err
	}

	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return "", fmt.Errorf("token expired")
	}
	return token.ID, nil
}

func getRequiredMasterTokenID() (string, error) {
	credential, err := getMasterTokenID()
	if err != nil {
		return "", err
	}

	if masterToken := os.Getenv("MASTER_TOKEN"); masterToken != "" && credential == masterToken {
		return credential, nil
	}

	conn, err := currentDB()
	if err != nil {
		return "", err
	}

	token, err := authz.FindTokenByValue(conn, credential)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		return "", fmt.Errorf("master token required")
	}
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return "", fmt.Errorf("token expired")
	}
	if !token.IsMaster {
		return "", fmt.Errorf("master token required")
	}
	return token.ID, nil
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

		token, err := getAuthTokenID()
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
		token, err := getAuthTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.CreateDatabase(dto.DatabaseCreateRequest{
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
	Use:   "get [id-or-name]",
	Short: "get database details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getAuthTokenID()
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
	Use:   "update [id-or-name]",
	Short: "update a database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		token, err := getAuthTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		database, err := svc.UpdateDatabase(args[0], dto.DatabaseUpdateRequest{
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
	Use:   "delete [id-or-name]",
	Short: "delete a database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		token, err := getAuthTokenID()
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

var dbImportCmd = &cobra.Command{
	Use:   "import --file schema.yaml",
	Short: "import a database from YAML file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("--file is required")
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		token, err := getRequiredMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewDatabaseService(db.DB())
		result, err := svc.ImportYAML(data, token)
		if err != nil {
			return err
		}
		return printJSON(result)
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbGetCmd)
	dbCmd.AddCommand(dbUpdateCmd)
	dbCmd.AddCommand(dbDeleteCmd)
	dbCmd.AddCommand(dbImportCmd)

	dbCreateCmd.Flags().StringP("description", "d", "", "database description")
	dbUpdateCmd.Flags().StringP("name", "n", "", "new name")
	dbUpdateCmd.Flags().StringP("description", "d", "", "new description")
	dbImportCmd.Flags().StringP("file", "f", "", "path to YAML file")
}
