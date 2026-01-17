package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dtek-bot",
	Short: "DTEK Emergency Alert Bot",
	Long:  `Бот для автоматичного відстеження та сповіщення про відключення електроенергії на сайті ДТЕК.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
