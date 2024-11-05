package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lpernett/godotenv"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type PingTask struct {
	stopCh         chan bool
	lastReport     time.Time
	lastStatusCode map[string]int
}

var pingTasks = make(map[int64]*PingTask)
var mu sync.Mutex

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Printf("Error loading dotenv: %s\n", err)
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	bot, err := telego.NewBot(botToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %s", err)
	}

	updates, _ := bot.UpdatesViaLongPolling(nil)
	bh, _ := th.NewBotHandler(bot, updates)

	defer bh.Stop()
	defer bot.StopLongPolling()

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID
		text := update.Message.Text

		args := strings.Fields(text)[1:]
		if len(args) == 0 {
			_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Пожалуйста, укажите URL-ы для пинга."))
			return
		}

		mu.Lock()
		if task, exists := pingTasks[chatID]; exists {
			close(task.stopCh)
		}

		stopCh := make(chan bool)
		task := &PingTask{
			stopCh:         stopCh,
			lastReport:     time.Now(),
			lastStatusCode: make(map[string]int),
		}
		pingTasks[chatID] = task
		mu.Unlock()

		go func(chatID int64, urls []string, task *PingTask) {
			for {
				select {
				case <-task.stopCh:
					return
				default:
					var results []string
					sendDailyReport := false

					for _, url := range urls {
						status, code := pingURL(url)
						results = append(results, status)

						if code != http.StatusOK {
							_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), fmt.Sprintf("⚠️ Alert! %s returned status code %d", url, code)))
						}

						if time.Since(task.lastReport).Hours() >= 24 {
							sendDailyReport = true
						}
					}

					if sendDailyReport {
						_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), strings.Join(results, "\n")))
						task.lastReport = time.Now()
					}

					time.Sleep(1 * time.Hour)
				}
			}
		}(chatID, args, task)

		_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Начат ежечасный пинг указанных URL-ов. Отправьте /cancel для остановки."))
	}, th.CommandEqual("ping"))

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID

		mu.Lock()
		if task, exists := pingTasks[chatID]; exists {
			close(task.stopCh) // Останавливаем задачу
			delete(pingTasks, chatID)
			_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Пинг остановлен."))
		} else {
			_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Активного пинга не найдено."))
		}
		mu.Unlock()
	}, th.CommandEqual("cancel"))

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID
		helpMessage := "Используйте /ping <URL1> <URL2> ... <URLn> для начала пинга сайтов. Используйте /cancel для остановки."
		_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), helpMessage))
	}, th.CommandEqual("help"))

	bh.Start()
}
