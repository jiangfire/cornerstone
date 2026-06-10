package cli

import (
	"fmt"

	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
)

var tableCmd = &cobra.Command{
	Use:   "table",
	Short: "table management",
	Long:  `Manage Cornerstone table resources. Supports list, create, get, update, delete subcommands.`,
}

var tableListCmd = &cobra.Command{
	Use:   "list [database-id-or-name]",
	Short: "list all tables in a database",
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
		svc := services.NewTableService(db.DB())
		tables, err := svc.ListTables(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(tables)
	},
}

var tableCreateCmd = &cobra.Command{
	Use:   "create [database-id-or-name] [name]",
	Short: "create a table in a database",
	Args:  cobra.ExactArgs(2),
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
		svc := services.NewTableService(db.DB())
		table, err := svc.CreateTable(services.CreateTableRequest{
			DatabaseID:  args[0],
			Name:        args[1],
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(table)
	},
}

var tableGetCmd = &cobra.Command{
	Use:   "get [id-or-name]",
	Short: "get table details",
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
		svc := services.NewTableService(db.DB())
		table, err := svc.GetTable(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(table)
	},
}

var tableUpdateCmd = &cobra.Command{
	Use:   "update [id-or-name]",
	Short: "update a table",
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
		svc := services.NewTableService(db.DB())
		table, err := svc.UpdateTable(args[0], services.UpdateTableRequest{
			Name:        name,
			Description: desc,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(table)
	},
}

var tableDeleteCmd = &cobra.Command{
	Use:   "delete [id-or-name]",
	Short: "delete a table",
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
		svc := services.NewTableService(db.DB())
		if err := svc.DeleteTable(args[0], token); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]interface{}{"id": args[0], "deleted": true})
		}
		fmt.Println("table deleted")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tableCmd)
	tableCmd.AddCommand(tableListCmd)
	tableCmd.AddCommand(tableCreateCmd)
	tableCmd.AddCommand(tableGetCmd)
	tableCmd.AddCommand(tableUpdateCmd)
	tableCmd.AddCommand(tableDeleteCmd)

	tableCreateCmd.Flags().StringP("description", "d", "", "table description")
	tableUpdateCmd.Flags().StringP("name", "n", "", "new name")
	tableUpdateCmd.Flags().StringP("description", "d", "", "new description")
}
