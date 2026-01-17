package notifier

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Notifier interface {
	SendPhoto(chatID int64, filePath string, caption string) (int, error)
	SendPhotoReply(chatID int64, replyMessageID int, filePath string, caption string) (int, error)
	EditPhoto(chatID int64, messageID int, filePath string, caption string) error
	EditCaption(chatID int64, messageID int, caption string) error
}

type telegramNotifier struct {
	bot *tgbotapi.BotAPI
}

func NewTelegramNotifier(token string) (Notifier, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &telegramNotifier{bot: bot}, nil
}

func (n *telegramNotifier) SendPhoto(chatID int64, filePath string, caption string) (int, error) {
	msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(filePath))
	msg.Caption = caption
	msg.ParseMode = tgbotapi.ModeHTML
	res, err := n.bot.Send(msg)
	if err != nil {
		return 0, err
	}
	return res.MessageID, nil
}

func (n *telegramNotifier) SendPhotoReply(chatID int64, replyMessageID int, filePath string, caption string) (int, error) {
	msg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(filePath))
	msg.ReplyToMessageID = replyMessageID
	msg.Caption = caption
	msg.ParseMode = tgbotapi.ModeHTML
	res, err := n.bot.Send(msg)
	if err != nil {
		return 0, err
	}
	return res.MessageID, nil
}

func (n *telegramNotifier) EditPhoto(chatID int64, messageID int, filePath string, caption string) error {
	photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(filePath))
	photo.Caption = caption
	photo.ParseMode = tgbotapi.ModeHTML
	editMedia := tgbotapi.EditMessageMediaConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    chatID,
			MessageID: messageID,
		},
		Media: photo,
	}
	_, err := n.bot.Send(editMedia)
	return err
}

func (n *telegramNotifier) EditCaption(chatID int64, messageID int, caption string) error {
	editMsg := tgbotapi.NewEditMessageCaption(chatID, messageID, caption)
	editMsg.ParseMode = tgbotapi.ModeHTML
	_, err := n.bot.Send(editMsg)
	return err
}
