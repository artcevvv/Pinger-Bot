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
	lastStatusCode int
}

type checkedURL struct {
	url    string
	status string
	code   int
}

var (
	pingTasks = make(map[int64]map[string]*PingTask)
	mu        sync.Mutex
)

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
		messageText := "Привет! Я- бот для проверки статуса доступности сайтов. \nДоступные команды:\n/ping, /help, /running\n\nСоздан @artcevvv для команды SunITy\n"

		_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), messageText))
	}, th.CommandEqual("start"))

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID
		text := update.Message.Text
		urls := strings.Fields(text)[1:]

		if len(urls) == 0 {
			_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Пожалуйста, укажите URL-ы для проверки."))
			return
		}

		mu.Lock()
		if _, exists := pingTasks[chatID]; !exists {
			pingTasks[chatID] = make(map[string]*PingTask)
		}
		mu.Unlock()

		for _, url := range urls {
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "https://" + url
			}

			mu.Lock()
			if _, exists := pingTasks[chatID][url]; exists {
				_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), fmt.Sprintf("Пинг уже запущен для %s", url)))
				mu.Unlock()
				continue
			}

			stopCh := make(chan bool)
			task := &PingTask{
				stopCh:         stopCh,
				lastReport:     time.Now(),
				lastStatusCode: http.StatusOK,
			}

			pingTasks[chatID][url] = task
			mu.Unlock()

			go startPinging(bot, chatID, url, task)
		}

		// Send immediate status report
		mu.Lock()
		var allResults []PingResult
		if tasks, exists := pingTasks[chatID]; exists {
			for taskURL := range tasks {
				result := pingURL(taskURL)
				allResults = append(allResults, result)
			}
		}
		mu.Unlock()

		// Format and send initial results
		formattedResults := formatPingResults(allResults)
		_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), formattedResults))

		_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Начат пинг указанных URL-ов. \n\nОтправьте /cancel <URL> для остановки конкретного пинга.\n\nДля проверки процессов- используйте /running"))
	}, th.CommandEqual("ping"))

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID
		args := strings.Fields(update.Message.Text)[1:]

		mu.Lock()
		defer mu.Unlock()

		if len(args) == 0 {
			if tasks, exists := pingTasks[chatID]; exists && len(tasks) > 0 {
				for url, task := range tasks {
					close(task.stopCh)
					delete(pingTasks[chatID], url)
				}
				_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Все пинги остановлены."))
			} else {
				_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Активного пинга не найдено."))
			}
		} else {
			url := args[0]
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "https://" + url
			}

			if task, exists := pingTasks[chatID][url]; exists {
				close(task.stopCh)
				delete(pingTasks[chatID], url)
				_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), fmt.Sprintf("Пинг остановлен для %s", url)))
			} else {
				_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Активного пинга для указанного URL не найдено."))
			}
		}
	}, th.CommandEqual("cancel"))

	// Running pings status command
	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID

		mu.Lock()
		defer mu.Unlock()

		if len(pingTasks[chatID]) == 0 {
			_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Нет активных пингов."))
			return
		}

		var runningUrls []string
		for url := range pingTasks[chatID] {
			runningUrls = append(runningUrls, url)
		}
		_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), "Активные пинги:\n"+strings.Join(runningUrls, "\n")))
	}, th.CommandEqual("running"))

	// Help command
	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID
		helpMessage := "Используйте /ping <URL1> <URL2> ... для начала пинга сайтов. Используйте /cancel <URL> для остановки определенного URL. Используйте /running для просмотра активных пингов."
		_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), helpMessage))
	}, th.CommandEqual("help"))

	bh.Start()
}

func startPinging(bot *telego.Bot, chatID int64, url string, task *PingTask) {
	for {
		select {
		case <-task.stopCh:
			return
		default:
			result := pingURL(url)
			
			if result.StatusCode != http.StatusOK && result.StatusCode != task.lastStatusCode {
				_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), fmt.Sprintf("⚠️ Alert! %s вернул статус %d", url, result.StatusCode)))
			}
			task.lastStatusCode = result.StatusCode

			if time.Since(task.lastReport).Minutes() >= 30 {
				// Collect all results for this chat
				mu.Lock()
				var allResults []PingResult
				if tasks, exists := pingTasks[chatID]; exists {
					for taskURL := range tasks {
						result := pingURL(taskURL)
						allResults = append(allResults, result)
					}
				}
				mu.Unlock()

				// Format and send all results in one message
				formattedResults := formatPingResults(allResults)
				_, _ = bot.SendMessage(tu.Message(tu.ID(chatID), formattedResults))
				task.lastReport = time.Now()
			}

			time.Sleep(30 * time.Minute)
		}
	}
}
