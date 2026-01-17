package cmd

import (
	"context"
	"dtek-emergency-alert/internal/app"
	"dtek-emergency-alert/internal/config"
	"dtek-emergency-alert/internal/notifier"
	"dtek-emergency-alert/internal/scraper"
	"dtek-emergency-alert/internal/storage"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	checkInterval int
)

var botCheckingCmd = &cobra.Command{
	Use:   "bot-checking",
	Short: "Запустити бота для перевірки відключень",
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stdout, "[BOT] ", log.LstdFlags|log.Lshortfile)

		cfg := config.Load(checkInterval)
		if err := cfg.Validate(); err != nil {
			logger.Fatalf("Configuration error: %v", err)
		}

		time.Local, _ = time.LoadLocation(cfg.TimeLocation)

		err := os.MkdirAll("data", 0755)
		if err != nil {
			logger.Fatal(err)
		}

		s := scraper.NewScraper(logger)
		st := storage.NewFileStorage(cfg.PrevFilePath)
		n, err := notifier.NewTelegramNotifier(cfg.TelegramBotToken)
		if err != nil {
			logger.Fatal(err)
		}

		botApp := app.NewApp(cfg, s, st, n, logger)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		logger.Println("Starting DTEK Alert Bot (bot-checking)...")
		botApp.Run(ctx)
		logger.Println("Bot stopped gracefully.")
	},
}

func init() {
	botCheckingCmd.Flags().IntVarP(&checkInterval, "check-interval", "i", 300, "Інтервал перевірки в секундах")
	rootCmd.AddCommand(botCheckingCmd)
}
