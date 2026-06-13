package cli

import (
	"fmt"

	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/dto"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
)

var fieldCmd = &cobra.Command{
	Use:   "field",
	Short: "field management",
	Long:  `Manage Cornerstone field resources. Supports list, create, get, update, delete subcommands.`,
}

var fieldListCmd = &cobra.Command{
	Use:   "list [table-id-or-name]",
	Short: "list all fields in a table",
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
		svc := services.NewFieldService(db.DB())
		fields, err := svc.ListFields(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(fields)
	},
}

var fieldCreateCmd = &cobra.Command{
	Use:   "create [table-id-or-name] [name] [type]",
	Short: "create a field in a table",
	Long: `Create a field in a table. Supported types:
  string, text, number, boolean, date, datetime, file, json, list`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		desc, _ := cmd.Flags().GetString("description")
		required, _ := cmd.Flags().GetBool("required")
		options, _ := cmd.Flags().GetString("options")
		token, err := getAuthTokenID()
		if err != nil {
			return err
		}
		svc := services.NewFieldService(db.DB())
		field, err := svc.CreateField(dto.FieldCreateRequest{
			TableID:     args[0],
			Name:        args[1],
			Type:        args[2],
			Description: desc,
			Required:    required,
			Options:     options,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(field)
	},
}

var fieldGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "get field details",
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
		svc := services.NewFieldService(db.DB())
		field, err := svc.GetField(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(field)
	},
}

var fieldUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "update a field",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		name, _ := cmd.Flags().GetString("name")
		fieldType, _ := cmd.Flags().GetString("type")
		desc, _ := cmd.Flags().GetString("description")
		required, _ := cmd.Flags().GetBool("required")
		options, _ := cmd.Flags().GetString("options")
		token, err := getAuthTokenID()
		if err != nil {
			return err
		}
		svc := services.NewFieldService(db.DB())
		field, err := svc.UpdateField(args[0], dto.FieldUpdateRequest{
			Name:        name,
			Type:        fieldType,
			Description: desc,
			Required:    required,
			Options:     options,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(field)
	},
}

var fieldDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "delete a field",
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
		svc := services.NewFieldService(db.DB())
		if err := svc.DeleteField(args[0], token); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]interface{}{"id": args[0], "deleted": true})
		}
		fmt.Println("field deleted")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fieldCmd)
	fieldCmd.AddCommand(fieldListCmd)
	fieldCmd.AddCommand(fieldCreateCmd)
	fieldCmd.AddCommand(fieldGetCmd)
	fieldCmd.AddCommand(fieldUpdateCmd)
	fieldCmd.AddCommand(fieldDeleteCmd)

	fieldCreateCmd.Flags().StringP("description", "d", "", "field description")
	fieldCreateCmd.Flags().BoolP("required", "r", false, "mark field as required")
	fieldCreateCmd.Flags().StringP("options", "o", "", "options (comma-separated)")

	fieldUpdateCmd.Flags().StringP("name", "n", "", "new name")
	fieldUpdateCmd.Flags().StringP("type", "t", "", "new type")
	fieldUpdateCmd.Flags().StringP("description", "d", "", "new description")
	fieldUpdateCmd.Flags().BoolP("required", "r", false, "mark field as required")
	fieldUpdateCmd.Flags().StringP("options", "o", "", "options (comma-separated)")
}
