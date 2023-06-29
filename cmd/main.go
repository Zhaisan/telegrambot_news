package cmd

import (
	"context"
	"errors"
	"github.com/Zhaisan/telegrambot_news/internal/config"
	"github.com/Zhaisan/telegrambot_news/internal/fetcher"
	"github.com/Zhaisan/telegrambot_news/internal/notifier"
	"github.com/Zhaisan/telegrambot_news/internal/storage"
	"github.com/Zhaisan/telegrambot_news/internal/summary"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jmoiron/sqlx"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	botAPI, err := tgbotapi.NewBotAPI(config.Get().TelegramBotToken)
	if err != nil {
		log.Printf("failed to create bot: %v", err)
		return
	}

	db, err := sqlx.Connect("postgres", config.Get().DatabaseDSN)
	if err != nil {
		log.Printf("failed to connect to database: %v", err)
		return
	}
	defer db.Close()

	var (
		articleStorage = storage.NewArticleStorage(db)
		sourceStorage  = storage.NewSourceStorage(db)
		fetcher        = fetcher.New(articleStorage, sourceStorage, config.Get().FetchInterval, config.Get().FilterKeywords)
		notifier       = notifier.New(
			articleStorage,
			summary.NewOpenAISummarizer(config.Get().OpenAIKey, config.Get().OpenAIPrompt),
			botAPI,
			config.Get().NotificationInterval,
			2*config.Get().FetchInterval,
			config.Get().TelegramChannelID)
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func(ctx context.Context) {
		if err := fetcher.Start(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("failed to start fetch: %v", err)
				return
			}

			log.Println("fetcher stopped")
		}
	}(ctx)

	//go func(ctx context.Context)
}
