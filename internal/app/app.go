package app

import (
	"context"
	"dtek-emergency-alert/internal/config"
	"dtek-emergency-alert/internal/models"
	"dtek-emergency-alert/internal/notifier"
	"dtek-emergency-alert/internal/scraper"
	"dtek-emergency-alert/internal/storage"
	"fmt"
	"log"
	"time"
)

type App struct {
	cfg      config.Config
	scraper  scraper.Scraper
	storage  storage.Storage
	notifier notifier.Notifier
	logger   *log.Logger
}

func NewApp(cfg config.Config, scraper scraper.Scraper, storage storage.Storage, notifier notifier.Notifier, logger *log.Logger) *App {
	return &App{
		cfg:      cfg,
		scraper:  scraper,
		storage:  storage,
		notifier: notifier,
		logger:   logger,
	}
}

func (a *App) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			a.logger.Println("Shutting down bot...")
			return
		default:
			a.logger.Println("Checking for updates...")
			if err := a.check(); err != nil {
				a.logger.Printf("Error checking updates: %v", err)
			}

			select {
			case <-ctx.Done():
				a.logger.Println("Shutting down bot...")
				return
			case <-time.After(a.cfg.CheckInterval):
			}
		}
	}
}

func (a *App) check() error {
	currentOutage, err := a.scraper.ScrapCurrentOutage(a.cfg.Street, a.cfg.House, a.cfg.ScreenshotPath)
	if err != nil {
		return fmt.Errorf("scrubbing error: %w", err)
	}

	prevData, err := a.storage.Load()
	if err != nil {
		a.logger.Printf("could not load prev data: %v", err)
	}

	if currentOutage == nil || !currentOutage.ShowCurOutage || currentOutage.Text == "" {
		a.logger.Println("No outage reported.")
		return a.handleNoOutage(prevData, currentOutage)
	}

	a.logger.Printf("debug %+v %+v", prevData, currentOutage)
	if a.isDataIdentical(prevData, currentOutage) {
		a.logger.Println("No updates to send (data identical to previous).")
		return nil
	}

	caption := a.formatDtekMessage(
		currentOutage.Text,
		currentOutage.StartDate,
		currentOutage.EndDate,
		currentOutage.UpdateTimestamp,
	)

	if prevData != nil && prevData.LastMessageID != 0 {
		isEndDateChanged := prevData.PrevEndDate != nil && !prevData.PrevEndDate.Equal(currentOutage.EndDate)

		if isEndDateChanged {
			a.logger.Println("EndDate changed. Sending new message as reply.")

			// Strikethrough previous message
			oldCaption := "<del>" + a.formatDtekMessage(
				*prevData.PrevText,
				*prevData.PrevStartDate,
				*prevData.PrevEndDate,
				*prevData.PrevUpdateTimestamp,
			) + "</del>\n\n<b>Час завершення змінено. Дивіться нове повідомлення нижче.</b>"
			err := a.notifier.EditCaption(a.cfg.ChatID, prevData.LastMessageID, oldCaption)
			if err != nil {
				a.logger.Printf("error editing old message: %v", err)
			}

			newCaption := caption + "\n\n<b>Час завершення змінено.</b>"
			msgID, err := a.notifier.SendPhotoReply(a.cfg.ChatID, prevData.LastMessageID, a.cfg.ScreenshotPath, newCaption)
			if err != nil {
				a.logger.Printf("error sending reply message: %v", err)
				// If reply fails, try sending a normal photo
				msgID, err = a.notifier.SendPhoto(a.cfg.ChatID, a.cfg.ScreenshotPath, caption)
				if err != nil {
					return fmt.Errorf("error sending message: %w", err)
				}
			}

			return a.storage.Save(models.SavedInfo{
				LastMessageID:       msgID,
				PrevText:            &currentOutage.Text,
				PrevUpdateTimestamp: &currentOutage.UpdateTimestamp,
				PrevStartDate:       &currentOutage.StartDate,
				PrevEndDate:         &currentOutage.EndDate,
			})
		}

		a.logger.Println("Updating existing message.")
		lastMessageID := prevData.LastMessageID

		err = a.notifier.EditPhoto(a.cfg.ChatID, prevData.LastMessageID, a.cfg.ScreenshotPath, caption)
		if err != nil {
			a.logger.Printf("update message error (might be too old or deleted): %v", err)
			// Reset state to try sending a new message next time

			a.logger.Println("Sending new message, with reply.")
			lastMessageID, err = a.notifier.SendPhotoReply(a.cfg.ChatID, prevData.LastMessageID, a.cfg.ScreenshotPath, caption)

			if err != nil {
				_ = a.storage.Save(models.SavedInfo{})

				return fmt.Errorf("error sending message: %w", err)
			}
		}

		return a.storage.Save(models.SavedInfo{
			LastMessageID:       lastMessageID,
			PrevText:            &currentOutage.Text,
			PrevUpdateTimestamp: &currentOutage.UpdateTimestamp,
			PrevStartDate:       &currentOutage.StartDate,
			PrevEndDate:         &currentOutage.EndDate,
		})
	}

	a.logger.Println("Sending new message.")
	msgID, err := a.notifier.SendPhoto(a.cfg.ChatID, a.cfg.ScreenshotPath, caption)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	a.logger.Println("Created new message.")
	return a.storage.Save(models.SavedInfo{
		LastMessageID:       msgID,
		PrevText:            &currentOutage.Text,
		PrevUpdateTimestamp: &currentOutage.UpdateTimestamp,
		PrevStartDate:       &currentOutage.StartDate,
		PrevEndDate:         &currentOutage.EndDate,
	})
}

func (a *App) handleNoOutage(prevData *models.SavedInfo, currentOutage *models.Outage) error {
	if prevData != nil && prevData.LastMessageID != 0 {
		a.logger.Println("Outage ended. Updating prev message.")

		// Use currentOutage data if available for formatting, even if not shown
		var text string
		var start, end, updated time.Time
		text = *prevData.PrevText
		start = *prevData.PrevStartDate
		end = *prevData.PrevEndDate
		updated = *prevData.PrevUpdateTimestamp

		caption := "<del>" + a.formatDtekMessage(text, start, end, updated) + "</del>\n\n<b>Відключення завершено або інформація відсутня.</b>"

		err := a.notifier.EditCaption(a.cfg.ChatID, prevData.LastMessageID, caption)
		if err != nil {
			a.logger.Printf("Error editing prev message: %v", err)
		}

		return a.storage.Save(models.SavedInfo{})
	}
	return nil
}

func (a *App) isDataIdentical(prevData *models.SavedInfo, currentOutage *models.Outage) bool {
	if prevData == nil || currentOutage == nil {
		return false
	}
	return prevData.PrevUpdateTimestamp != nil && prevData.PrevUpdateTimestamp.Equal(currentOutage.UpdateTimestamp) &&
		prevData.PrevText != nil && *prevData.PrevText == currentOutage.Text &&
		prevData.PrevStartDate != nil && prevData.PrevStartDate.Equal(currentOutage.StartDate) &&
		prevData.PrevEndDate != nil && prevData.PrevEndDate.Equal(currentOutage.EndDate)
}

func (a *App) formatDtekMessage(text string, startDate, endDate, updateTimestamp time.Time) string {
	return fmt.Sprintf(
		"<b>Повідомлення від ДТЕК</b>\n\n<blockquote>%s</blockquote>\n\n<b>Період:</b> з %s по %s\n<b>Оновлено:</b> %s",
		text,
		startDate.Format(a.cfg.TimeFormat),
		endDate.Format(a.cfg.TimeFormat),
		updateTimestamp.Format(a.cfg.TimeFormat),
	)
}
