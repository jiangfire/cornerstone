package cli

import (
	"fmt"

	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
)

var fieldCmd = &cobra.Command{
	Use:   "field",
	Short: "字段管理",
	Long:  `管理 Cornerstone 字段资源。支持 list、create、get、update、delete 子命令。`,
}

var fieldListCmd = &cobra.Command{
	Use:   "list [table-id]",
	Short: "列出表下的所有字段",
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
		svc := services.NewFieldService(db.DB())
		fields, err := svc.ListFields(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(fields)
	},
}

var fieldCreateCmd = &cobra.Command{
	Use:   "create [table-id] [name] [type]",
	Short: "在表中创建字段",
	Long: `在表中创建字段。支持的字段类型：
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
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewFieldService(db.DB())
		field, err := svc.CreateField(services.CreateFieldRequest{
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
	Short: "获取字段详情",
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
	Short: "更新字段",
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
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewFieldService(db.DB())
		field, err := svc.UpdateField(args[0], services.UpdateFieldRequest{
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
	Short: "删除字段",
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
		svc := services.NewFieldService(db.DB())
		if err := svc.DeleteField(args[0], token); err != nil {
			return err
		}
		fmt.Println("字段已删除")
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

	fieldCreateCmd.Flags().StringP("description", "d", "", "字段描述")
	fieldCreateCmd.Flags().BoolP("required", "r", false, "是否必填")
	fieldCreateCmd.Flags().StringP("options", "o", "", "选项（逗号分隔）")

	fieldUpdateCmd.Flags().StringP("name", "n", "", "新名称")
	fieldUpdateCmd.Flags().StringP("type", "t", "", "新类型")
	fieldUpdateCmd.Flags().StringP("description", "d", "", "新描述")
	fieldUpdateCmd.Flags().BoolP("required", "r", false, "是否必填")
	fieldUpdateCmd.Flags().StringP("options", "o", "", "选项（逗号分隔）")
}
