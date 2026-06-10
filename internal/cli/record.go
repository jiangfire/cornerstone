package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	appdb "github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "record management",
	Long:  `Manage Cornerstone record resources. Supports list, create, get, update, delete, batch subcommands.`,
}

func recordForJSON(record *models.Record) (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	if record.Data != "" {
		if err := json.Unmarshal([]byte(record.Data), &payload); err != nil {
			return nil, fmt.Errorf("invalid stored record data: %w", err)
		}
	}

	return map[string]interface{}{
		"id":       record.ID,
		"table_id": record.TableID,
		"data":     payload,
		"version":  record.Version,
	}, nil
}

func printRecordJSON(record *models.Record) error {
	payload, err := recordForJSON(record)
	if err != nil {
		return err
	}
	return printJSON(payload)
}

func printRecordsJSON(records []*models.Record) error {
	payload := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		item, err := recordForJSON(record)
		if err != nil {
			return err
		}
		payload = append(payload, item)
	}
	return printJSON(payload)
}

var recordListCmd = &cobra.Command{
	Use:   "list [table-id]",
	Short: "list records in a table",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		filter, _ := cmd.Flags().GetString("filter")
		token, err := getAuthTokenID()
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
	Short: "create a record",
	Long: `Create a record. data-json is JSON field data, e.g.:
  '{"name": "test", "age": 25}'`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(args[1]), &data); err != nil {
			return fmt.Errorf("invalid JSON data: %w", err)
		}

		token, err := getAuthTokenID()
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
		return printRecordJSON(record)
	},
}

var recordGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "get record details",
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
		svc := services.NewRecordService(db.DB())
		record, err := svc.GetRecord(args[0], token, "")
		if err != nil {
			return err
		}
		return printJSON(record)
	},
}

var recordUpdateCmd = &cobra.Command{
	Use:   "update [id] [data-json]",
	Short: "update a record",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(args[1]), &data); err != nil {
			return fmt.Errorf("invalid JSON data: %w", err)
		}

		version, _ := cmd.Flags().GetInt("version")
		token, err := getAuthTokenID()
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
		return printRecordJSON(record)
	},
}

var recordDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "delete a record",
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
		svc := services.NewRecordService(db.DB())
		if err := svc.DeleteRecord(args[0], token); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]interface{}{"id": args[0], "deleted": true})
		}
		fmt.Println("record deleted")
		return nil
	},
}

var recordBatchCmd = &cobra.Command{
	Use:   "batch [table-id] [data-json] [count]",
	Short: "batch create records",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureDB(); err != nil {
			return err
		}
		defer func() { _ = appdb.CloseDB() }()

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(args[1]), &data); err != nil {
			return fmt.Errorf("invalid JSON data: %w", err)
		}

		count, err := strconv.Atoi(args[2])
		if err != nil || count < 1 || count > 100 {
			return fmt.Errorf("count must be between 1 and 100")
		}

		token, err := getAuthTokenID()
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
		if !jsonOutput {
			fmt.Printf("created %d records\n", len(records))
		}
		return printRecordsJSON(records)
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

	recordListCmd.Flags().IntP("limit", "l", 20, "page size")
	recordListCmd.Flags().IntP("offset", "o", 0, "offset")
	recordListCmd.Flags().StringP("filter", "f", "", "filter condition (JSON)")

	recordUpdateCmd.Flags().IntP("version", "v", 0, "optimistic lock version")
}
