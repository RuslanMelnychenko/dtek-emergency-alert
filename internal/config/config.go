package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Street           string
	House            string
	TelegramBotToken string
	ChatID           int64
	CheckInterval    time.Duration
	ScreenshotPath   string
	PrevFilePath     string
	TimeFormat       string
	TimeLocation     string
}

func Load(checkIntervalSeconds int) Config {
	v := viper.New()

	v.SetDefault("SCREENSHOT_PATH", "data/currentOutstage.jpeg")
	v.SetDefault("PREV_FILE_PATH", "data/prevData.json")
	v.SetDefault("TIME_FORMAT", "15:04 02.01.2006")
	v.SetDefault("TIME_LOCATION", "Europe/Kyiv")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return Config{
		Street:           v.GetString("STREET"),
		House:            v.GetString("HOUSE"),
		TelegramBotToken: v.GetString("TELEGRAM_BOT_TOKEN"),
		ChatID:           v.GetInt64("TELEGRAM_CHAT_ID"),
		CheckInterval:    time.Duration(checkIntervalSeconds) * time.Second,
		ScreenshotPath:   v.GetString("SCREENSHOT_PATH"),
		PrevFilePath:     v.GetString("PREV_FILE_PATH"),
		TimeFormat:       v.GetString("TIME_FORMAT"),
		TimeLocation:     v.GetString("TIME_LOCATION"),
	}
}

func (c Config) Validate() error {
	if c.Street == "" {
		return fmt.Errorf("STREET is not set")
	}
	if c.House == "" {
		return fmt.Errorf("HOUSE is not set")
	}
	if c.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}
	if c.ChatID == 0 {
		return fmt.Errorf("TELEGRAM_CHAT_ID is not set")
	}
	return nil
}
