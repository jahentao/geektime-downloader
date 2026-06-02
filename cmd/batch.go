package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nicoxiang/geektime-downloader/internal/batch"
	"github.com/nicoxiang/geektime-downloader/internal/config"
	"github.com/nicoxiang/geektime-downloader/internal/geektime"
	"github.com/nicoxiang/geektime-downloader/internal/pkg/logger"
)

func init() {
	rootCmd.AddCommand(batchCmd)
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "批量下载所有已购课程",
	Long:  `自动获取所有已购课程列表并全部下载，无需交互式选择。`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		logger.Init(cfg.LogLevel)
		return config.ValidateConfig(&cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		readCookies := config.ReadCookiesFromInput(&cfg)
		client := geektime.NewClient(readCookies)
		return batch.BatchDownloader(cmd.Context(), &cfg, client)
	},
}
