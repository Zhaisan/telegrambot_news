package notifier

import (
	"context"
	"fmt"
	"github.com/Zhaisan/telegrambot_news/internal/botkit/markup"
	"github.com/Zhaisan/telegrambot_news/internal/model"
	"github.com/go-shiori/go-readability"
	"io"
	"net/http"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"time"
)

type ArticleProvider interface {
	AllNotPosted(ctx context.Context, since time.Time, limit uint64) ([]model.Article, error)
	MarkAsPosted(ctx context.Context, article model.Article) error
}

type Summarizer interface {
	Summarize(ctx context.Context, text string) (string, error)
}

type Notifier struct {
	articles         ArticleProvider
	summarizer       Summarizer
	bot              *tgbotapi.BotAPI
	sendInterval     time.Duration
	lookupTimeWindow time.Duration
	channelID        int64
}

func New(
	articleProvider ArticleProvider,
	summarizer Summarizer,
	bot *tgbotapi.BotAPI,
	sendInterval time.Duration,
	lookupTimeWindow time.Duration,
	channelID int64,
) *Notifier {
	return &Notifier{
		articles:         articleProvider,
		summarizer:       summarizer,
		bot:              bot,
		sendInterval:     sendInterval,
		lookupTimeWindow: lookupTimeWindow,
		channelID:        channelID,
	}
}

func (n *Notifier) SelectAndSendArticle(ctx context.Context) error {
	topOneArticles, err := n.articles.AllNotPosted(ctx, time.Now().Add(-n.lookupTimeWindow), 1)
	if err != nil {
		return err
	}

	if len(topOneArticles) == 0 {
		return nil
	}

	article := topOneArticles[0]

	summary, err := n.extractSummary(ctx, article)
	if err != nil {
		return err
	}

	if err := n.sendArticle(article, summary); err != nil {
		return err
	}

	return n.articles.MarkAsPosted(ctx, article)
}

func (n *Notifier) extractSummary(ctx context.Context, article model.Article) (string, error) {
	var r io.Reader

	if article.Summary != "" {
		r = strings.NewReader(article.Summary)
	} else {
		resp, err := http.Get(article.Link)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		r = resp.Body
	}

	doc, err := readability.FromReader(r, nil)
	if err != nil {
		return "", err
	}

	summary, err := n.summarizer.Summarize(ctx, cleanText(doc.TextContent))
	if err != nil {
		return "", err
	}

	return "\n\n" + summary, nil
}

func (n *Notifier) sendArticle(article model.Article, summary string) error {
	const msgFormat = "*%s*%s\n\n%s"

	msg := tgbotapi.NewMessage(n.channelID, fmt.Sprintf(msgFormat,
		markup.EscapeForMarkdown(article.Title),
		markup.EscapeForMarkdown(summary),
		markup.EscapeForMarkdown(article.Link),
	))
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	_, err := n.bot.Send(msg)
	if err != nil {
		return err
	}

	return nil
}

var redundantNewLines = regexp.MustCompile(`\n{3,}`)

func cleanText(text string) string {
	return redundantNewLines.ReplaceAllString(text, "\n")
}
