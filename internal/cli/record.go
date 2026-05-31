package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "记录管理",
	Long:  `管理 Cornerstone 记录资源。支持 list、create、get、update、delete、batch 子命令。`,
}

var recordListCmd = &cobra.Command{
	Use:   "list [table-id]",
	Short: "列出表下的记录",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer appdb.CloseDB()

		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		filter, _ := cmd.Flags().GetString("filter")
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewRecordService(db.DB())
		result, err := svc.ListRecords(services.QueryRequest{
			TableID: args[0],
			Limit:   limit,
			Offset:  offset,
			Filter:  filter,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(result)
	},
}

var recordCreateCmd = &cobra.Command{
	Use:   "create [table-id] [data-json]",
	Short: "创建记录",
	Long: `创建记录。data-json 是 JSON 格式的字段数据，如：
  '{"name": "test", "age": 25}'`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer appdb.CloseDB()

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(args[1]), &data); err != nil {
			return fmt.Errorf("无效的 JSON 数据: %w", err)
		}

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewRecordService(db.DB())
		record, err := svc.CreateRecord(services.CreateRecordRequest{
			TableID: args[0],
			Data:    data,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(record)
	},
}

var recordGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "获取记录详情",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer appdb.CloseDB()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewRecordService(db.DB())
		record, err := svc.GetRecord(args[0], token)
		if err != nil {
			return err
		}
		return printJSON(record)
	},
}

var recordUpdateCmd = &cobra.Command{
	Use:   "update [id] [data-json]",
	Short: "更新记录",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer appdb.CloseDB()

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(args[1]), &data); err != nil {
			return fmt.Errorf("无效的 JSON 数据: %w", err)
		}

		version, _ := cmd.Flags().GetInt("version")
		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewRecordService(db.DB())
		record, err := svc.UpdateRecord(args[0], services.UpdateRecordRequest{
			Data:    data,
			Version: version,
		}, token)
		if err != nil {
			return err
		}
		return printJSON(record)
	},
}

var recordDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "删除记录",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer appdb.CloseDB()

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewRecordService(db.DB())
		if err := svc.DeleteRecord(args[0], token); err != nil {
			return err
		}
		fmt.Println("记录已删除")
		return nil
	},
}

var recordBatchCmd = &cobra.Command{
	Use:   "batch [table-id] [data-json] [count]",
	Short: "批量创建记录",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer appdb.CloseDB()

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(args[1]), &data); err != nil {
			return fmt.Errorf("无效的 JSON 数据: %w", err)
		}

		count, err := strconv.Atoi(args[2])
		if err != nil || count < 1 || count > 100 {
			return fmt.Errorf("数量必须在1-100之间")
		}

		token, err := getMasterTokenID()
		if err != nil {
			return err
		}
		svc := services.NewRecordService(db.DB())
		records, err := svc.BatchCreateRecords(services.CreateRecordRequest{
			TableID: args[0],
			Data:    data,
		}, token, count)
		if err != nil {
			return err
		}
		fmt.Printf("成功创建 %d 条记录\n", len(records))
		return printJSON(records)
	},
}

func init() {
	rootCmd.AddCommand(recordCmd)
	recordCmd.AddCommand(recordListCmd)
	recordCmd.AddCommand(recordCreateCmd)
	recordCmd.AddCommand(recordGetCmd)
	recordCmd.AddCommand(recordUpdateCmd)
	recordCmd.AddCommand(recordDeleteCmd)
	recordCmd.AddCommand(recordBatchCmd)

	recordListCmd.Flags().IntP("limit", "l", 20, "每页数量")
	recordListCmd.Flags().IntP("offset", "o", 0, "偏移量")
	recordListCmd.Flags().StringP("filter", "f", "", "过滤条件(JSON)")

	recordUpdateCmd.Flags().IntP("version", "v", 0, "乐观锁版本号")
}
