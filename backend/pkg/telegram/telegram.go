package telegram

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

var client = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
}

// SendMessage отправляет HTML-сообщение в указанный чат через Telegram Bot API.
func SendMessage(botToken, chatID, text string) {
	if botToken == "" || chatID == "" {
		return
	}
	go func() {
		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
		params := url.Values{}
		params.Set("chat_id", chatID)
		params.Set("text", text)
		params.Set("parse_mode", "HTML")
		resp, err := client.PostForm(apiURL, params)
		if err != nil {
			log.Printf("[telegram] предупреждение: ошибка отправки сообщения: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			log.Printf("[telegram] предупреждение: API вернул статус %d", resp.StatusCode)
		}
	}()
}
