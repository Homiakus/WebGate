package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/process"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

// InlineButton defines an interactive inline button in Telegram chat.
type InlineButton struct {
	Text         string            `json:"text"`
	CallbackData string            `json:"callback_data,omitempty"`
	URL          string            `json:"url,omitempty"`
	WebApp       *TelegramWebAppURL `json:"web_app,omitempty"`
}

type TelegramWebAppURL struct {
	URL string `json:"url"`
}

// BotResponse is returned for both web emulation and real Telegram dispatches.
type BotResponse struct {
	Text           string           `json:"text"`
	InlineKeyboard [][]InlineButton `json:"inline_keyboard,omitempty"`
	Timestamp      time.Time        `json:"timestamp"`
}

// BotEvent logs Telegram interactions for observability in the dashboard.
type BotEvent struct {
	ID        string    `json:"id"`
	ChatID    int64     `json:"chat_id"`
	Username  string    `json:"username,omitempty"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// Telegram API structures
type tgUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type tgChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Username string `json:"username"`
}

type tgMessage struct {
	MessageID int       `json:"message_id"`
	From      tgUser    `json:"from"`
	Chat      tgChat    `json:"chat"`
	Date      int64     `json:"date"`
	Text      string    `json:"text"`
}

type tgCallbackQuery struct {
	ID      string    `json:"id"`
	From    tgUser    `json:"from"`
	Message tgMessage `json:"message"`
	Data    string    `json:"data"`
}

type tgUpdate struct {
	UpdateID      int              `json:"update_id"`
	Message       *tgMessage       `json:"message,omitempty"`
	CallbackQuery *tgCallbackQuery `json:"callback_query,omitempty"`
}

type tgGetUpdatesResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

type tgGetMeResponse struct {
	OK     bool   `json:"ok"`
	Result tgUser `json:"result"`
}

type AdminBot struct {
	mu           sync.RWMutex
	services     *registry.ServiceRegistry
	procManager  *process.ProcessManager
	adminChatIDs map[int64]bool
	botToken     string
	apiEndpoint  string
	botUsername  string
	botFirstName string
	isConnected  bool
	httpClient   *http.Client
	cancelPoll   context.CancelFunc
	events       []BotEvent
}

func NewAdminBot(services *registry.ServiceRegistry, procManager *process.ProcessManager) *AdminBot {
	bot := &AdminBot{
		services:     services,
		procManager:  procManager,
		adminChatIDs: make(map[int64]bool),
		apiEndpoint:  "https://api.telegram.org",
		httpClient:   &http.Client{Timeout: 35 * time.Second},
		events:       make([]BotEvent, 0),
	}
	// Default allow root admin chat 999001
	bot.adminChatIDs[999001] = true

	// Hook into process manager for proactive crash/exit alerts
	if procManager != nil {
		procManager.SetOnExitHook(func(serviceID string, pid int, exitCode int, err error) {
			bot.notifyProcessExit(serviceID, pid, exitCode, err)
		})
	}

	return bot
}

// SetAuthorizedAdmins sets the complete list of authorized Telegram chat IDs.
func (b *AdminBot) SetAuthorizedAdmins(chatIDs []int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.adminChatIDs = make(map[int64]bool)
	for _, id := range chatIDs {
		b.adminChatIDs[id] = true
	}
}

// AuthorizeAdmin registers a Telegram chat ID as an authorized administrator.
func (b *AdminBot) AuthorizeAdmin(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.adminChatIDs[chatID] = true
}

// IsAuthorized checks if the sender is an authorized administrator.
func (b *AdminBot) IsAuthorized(chatID int64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.adminChatIDs) == 0 {
		return true
	}
	return b.adminChatIDs[chatID]
}

// SetBotToken updates the Telegram bot token and starts long-polling if valid.
func (b *AdminBot) SetBotToken(token string) {
	b.mu.Lock()
	b.botToken = strings.TrimSpace(token)
	if b.cancelPoll != nil {
		b.cancelPoll()
		b.cancelPoll = nil
	}
	b.isConnected = false
	b.botUsername = ""
	b.mu.Unlock()

	if token != "" {
		ctx, cancel := context.WithCancel(context.Background())
		b.mu.Lock()
		b.cancelPoll = cancel
		b.mu.Unlock()
		go b.runPollingLoop(ctx)
	}
}

// GetStatus returns current live connection metadata for the dashboard.
func (b *AdminBot) GetStatus() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	chatList := make([]int64, 0, len(b.adminChatIDs))
	for id := range b.adminChatIDs {
		chatList = append(chatList, id)
	}

	return map[string]interface{}{
		"connected":        b.isConnected,
		"bot_username":     b.botUsername,
		"bot_name":         b.botFirstName,
		"bot_link":         fmt.Sprintf("https://t.me/%s", b.botUsername),
		"has_token":        b.botToken != "",
		"authorized_chats": chatList,
		"recent_events":    b.events,
	}
}

func (b *AdminBot) logEvent(chatID int64, username, action, details string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	event := BotEvent{
		ID:        time.Now().Format("20060102150405.000000"),
		ChatID:    chatID,
		Username:  username,
		Action:    action,
		Details:   details,
		Timestamp: time.Now().UTC(),
	}
	b.events = append(b.events, event)
	if len(b.events) > 50 {
		b.events = b.events[len(b.events)-50:]
	}
}

// notifyProcessExit sends a proactive emergency alert card to all admins when a service terminates.
func (b *AdminBot) notifyProcessExit(serviceID string, pid int, exitCode int, err error) {
	b.mu.RLock()
	token := b.botToken
	adminChats := make([]int64, 0, len(b.adminChatIDs))
	for id := range b.adminChatIDs {
		adminChats = append(adminChats, id)
	}
	b.mu.RUnlock()

	if token == "" || len(adminChats) == 0 {
		return
	}

	svc, _ := b.services.GetByID(serviceID)
	name := serviceID
	slug := serviceID
	if svc != nil {
		name = svc.Name
		slug = svc.Slug
	}

	statusHeader := "⚠️ *ПРОЦЕСС ЗАВЕРШИЛ РАБОТУ*"
	if exitCode != 0 || err != nil {
		statusHeader = "🚨 *ВНИМАНИЕ: СЕРВИС УПАЛ С ОШИБКОЙ*"
	}

	alertText := fmt.Sprintf("%s\n\n"+
		"• Сервис: *%s* (`%s`)\n"+
		"• PID: `%d`\n"+
		"• Код выхода: `%d`\n"+
		"• Время: `%s` UTC",
		statusHeader, name, slug, pid, exitCode, time.Now().UTC().Format("15:04:05 02.01.2006"))

	keyboard := [][]InlineButton{
		{
			{Text: fmt.Sprintf("🚀 Перезапустить %s", slug), CallbackData: "restart:" + serviceID},
			{Text: "📋 Хвост логов", CallbackData: "logs:" + serviceID},
		},
		{
			{Text: "📊 Все сервисы", CallbackData: "menu:services"},
		},
	}

	for _, chatID := range adminChats {
		b.sendTelegramMessage(token, chatID, alertText, keyboard, false)
	}
}

// runPollingLoop starts continuous long polling to the real Telegram Bot API.
func (b *AdminBot) runPollingLoop(ctx context.Context) {
	token := b.botToken
	if token == "" {
		return
	}

	// 1. Verify Bot Token via getMe
	getMeURL := fmt.Sprintf("%s/bot%s/getMe", b.apiEndpoint, token)
	resp, err := b.httpClient.Get(getMeURL)
	if err != nil {
		log.Printf("[Telegram Bot] Ошибка подключения к API: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var meResp tgGetMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&meResp); err != nil || !meResp.OK {
		log.Printf("[Telegram Bot] Недействительный Bot Token или ошибка ответа: %v\n", err)
		return
	}

	b.mu.Lock()
	b.isConnected = true
	b.botUsername = meResp.Result.Username
	b.botFirstName = meResp.Result.FirstName
	b.mu.Unlock()

	log.Printf("[Telegram Bot] 🟢 Успешно запущен реальный бот @%s (%s)\n", b.botUsername, b.botFirstName)

	// 2. Long Polling loop
	offset := 0
	for {
		select {
		case <-ctx.Done():
			log.Println("[Telegram Bot] Polling loop остановлен")
			return
		default:
		}

		pollURL := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&timeout=20", b.apiEndpoint, token, offset)
		req, _ := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
		pResp, err := b.httpClient.Do(req)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		var upResp tgGetUpdatesResponse
		bodyBytes, _ := io.ReadAll(pResp.Body)
		pResp.Body.Close()

		if err := json.Unmarshal(bodyBytes, &upResp); err != nil || !upResp.OK {
			time.Sleep(3 * time.Second)
			continue
		}

		for _, up := range upResp.Result {
			if up.UpdateID >= offset {
				offset = up.UpdateID + 1
			}

			if up.Message != nil && up.Message.Text != "" {
				b.handleIncomingTelegramMessage(token, up.Message)
			} else if up.CallbackQuery != nil {
				b.handleIncomingTelegramCallback(token, up.CallbackQuery)
			}
		}
	}
}

func (b *AdminBot) handleIncomingTelegramMessage(token string, msg *tgMessage) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)
	username := msg.From.Username
	if username == "" {
		username = msg.From.FirstName
	}

	b.logEvent(chatID, username, "MESSAGE", text)

	// Authorization check
	if !b.IsAuthorized(chatID) {
		deniedText := fmt.Sprintf("⛔ *Доступ Запрещен*\n\nВаш Telegram Chat ID: `%d`\nЭтот аккаунт (@%s) не добавлен в список администраторов WebGate.\n\n_Добавьте этот ID в панели администратора в Настройках сервера._", chatID, username)
		b.sendTelegramMessage(token, chatID, deniedText, nil, true)
		return
	}

	// Route command or persistent keyboard text
	var botResp BotResponse
	switch text {
	case "📊 Сервисы", "/services", "/status":
		botResp = b.servicesResponse()
	case "🔌 Порты", "/ports":
		botResp = b.portsResponse()
	case "⚡ Статус Шлюза", "/health":
		botResp = b.healthResponse()
	case "📦 Клиент", "/get_client", "/client":
		botResp = b.clientDeliveryResponse()
	case "ℹ️ Помощь", "/help", "/start":
		botResp = b.helpResponse()
	default:
		if strings.HasPrefix(text, "/logs ") {
			slug := strings.TrimSpace(strings.TrimPrefix(text, "/logs "))
			botResp = b.logsResponse(slug)
		} else {
			botResp = b.HandleCommand(chatID, text)
		}
	}

	b.sendTelegramMessage(token, chatID, botResp.Text, botResp.InlineKeyboard, true)
}

func (b *AdminBot) handleIncomingTelegramCallback(token string, cb *tgCallbackQuery) {
	chatID := cb.Message.Chat.ID
	data := cb.Data
	msgID := cb.Message.MessageID
	username := cb.From.Username

	b.logEvent(chatID, username, "CALLBACK", data)

	// Answer callback first
	b.answerCallbackQuery(token, cb.ID, "Обработка запроса...")

	if !b.IsAuthorized(chatID) {
		b.answerCallbackQuery(token, cb.ID, "⛔ Доступ запрещен")
		return
	}

	parts := strings.Split(data, ":")
	action := parts[0]
	targetID := ""
	if len(parts) > 1 {
		targetID = parts[1]
	}

	var botResp BotResponse
	switch action {
	case "menu":
		switch targetID {
		case "services":
			botResp = b.servicesResponse()
		case "ports":
			botResp = b.portsResponse()
		case "health":
			botResp = b.healthResponse()
		case "client":
			botResp = b.clientDeliveryResponse()
		default:
			botResp = b.helpResponse()
		}

	case "view":
		botResp = b.serviceDetailResponse(targetID)

	case "logs":
		botResp = b.logsResponse(targetID)

	case "start":
		b.editTelegramMessage(token, chatID, msgID, fmt.Sprintf("⏳ *Запуск сервиса %s...*\nВыделение порта и запуск процесса...", targetID), nil)
		time.Sleep(300 * time.Millisecond)
		botResp = b.startServiceCommand(targetID)

	case "stop":
		b.editTelegramMessage(token, chatID, msgID, fmt.Sprintf("⏳ *Остановка сервиса %s...*\nЗавершение процесса...", targetID), nil)
		time.Sleep(300 * time.Millisecond)
		botResp = b.stopServiceCommand(targetID)

	case "restart":
		b.editTelegramMessage(token, chatID, msgID, fmt.Sprintf("🔄 *Перезапуск сервиса %s...*\nОстановка и новый запуск...", targetID), nil)
		time.Sleep(400 * time.Millisecond)
		botResp = b.restartServiceCommand(targetID)

	case "confirm_stop":
		botResp = b.confirmActionResponse("stop", targetID, "Остановить процесс")

	case "refresh":
		botResp = b.servicesResponse()

	case "ports":
		botResp = b.portsResponse()

	case "health":
		botResp = b.healthResponse()

	default:
		botResp = BotResponse{Text: "Действие выполнено.", Timestamp: time.Now().UTC()}
	}

	b.editTelegramMessage(token, chatID, msgID, botResp.Text, botResp.InlineKeyboard)
}

// sendTelegramMessage sends a real message via Telegram Bot API with Reply + Inline Keyboards.
func (b *AdminBot) sendTelegramMessage(token string, chatID int64, text string, inlineKeyboards [][]InlineButton, includeReplyMenu bool) {
	if token == "" {
		return
	}

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	if len(inlineKeyboards) > 0 {
		payload["reply_markup"] = map[string]interface{}{
			"inline_keyboard": b.serializeInlineKeyboard(inlineKeyboards),
		}
	} else if includeReplyMenu {
		payload["reply_markup"] = map[string]interface{}{
			"keyboard": [][]map[string]string{
				{{"text": "📊 Сервисы"}, {"text": "⚡ Статус Шлюза"}},
				{{"text": "🔌 Порты"}, {"text": "📦 Клиент"}},
				{{"text": "ℹ️ Помощь"}},
			},
			"resize_keyboard": true,
			"persistent":      true,
		}
	}

	jsonBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/bot%s/sendMessage", b.apiEndpoint, token)
	_, _ = b.httpClient.Post(url, "application/json", bytes.NewReader(jsonBytes))
}

// editTelegramMessage edits an existing message in-place in Telegram chat.
func (b *AdminBot) editTelegramMessage(token string, chatID int64, messageID int, text string, inlineKeyboards [][]InlineButton) {
	if token == "" {
		return
	}

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	if len(inlineKeyboards) > 0 {
		payload["reply_markup"] = map[string]interface{}{
			"inline_keyboard": b.serializeInlineKeyboard(inlineKeyboards),
		}
	}

	jsonBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/bot%s/editMessageText", b.apiEndpoint, token)
	_, _ = b.httpClient.Post(url, "application/json", bytes.NewReader(jsonBytes))
}

func (b *AdminBot) serializeInlineKeyboard(rows [][]InlineButton) [][]map[string]interface{} {
	var tgRows [][]map[string]interface{}
	for _, row := range rows {
		var tgRow []map[string]interface{}
		for _, btn := range row {
			bMap := map[string]interface{}{"text": btn.Text}
			if btn.CallbackData != "" {
				bMap["callback_data"] = btn.CallbackData
			}
			if btn.URL != "" {
				bMap["url"] = btn.URL
			}
			if btn.WebApp != nil {
				bMap["web_app"] = map[string]string{"url": btn.WebApp.URL}
			}
			tgRow = append(tgRow, bMap)
		}
		tgRows = append(tgRows, tgRow)
	}
	return tgRows
}

func (b *AdminBot) answerCallbackQuery(token string, callbackID, text string) {
	if token == "" {
		return
	}
	payload := map[string]string{
		"callback_query_id": callbackID,
		"text":              text,
	}
	jsonBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/bot%s/answerCallbackQuery", b.apiEndpoint, token)
	_, _ = b.httpClient.Post(url, "application/json", bytes.NewReader(jsonBytes))
}

// HandleCommand processes commands sent by the administrator.
func (b *AdminBot) HandleCommand(chatID int64, rawText string) BotResponse {
	if !b.IsAuthorized(chatID) {
		return BotResponse{
			Text:      fmt.Sprintf("⛔ Access Denied. Chat ID %d is not authorized.", chatID),
			Timestamp: time.Now().UTC(),
		}
	}

	trimmed := strings.TrimSpace(rawText)
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return b.helpResponse()
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/start", "/help", "ℹ️ помощь":
		return b.helpResponse()

	case "/services", "/status", "/list", "📊 сервисы":
		return b.servicesResponse()

	case "/ports", "🔌 порты":
		return b.portsResponse()

	case "/health", "⚡ статус шлюза":
		return b.healthResponse()

	case "/client", "/get_client", "📦 клиент":
		return b.clientDeliveryResponse()

	case "/logs":
		if len(parts) < 2 {
			return BotResponse{Text: "Использование: /logs <slug_or_id>", Timestamp: time.Now().UTC()}
		}
		return b.logsResponse(parts[1])

	case "/start_service", "/run":
		if len(parts) < 2 {
			return BotResponse{Text: "Использование: /start_service <slug_or_id>", Timestamp: time.Now().UTC()}
		}
		return b.startServiceCommand(parts[1])

	case "/stop_service", "/kill", "/stop":
		if len(parts) < 2 {
			return BotResponse{Text: "Использование: /stop_service <slug_or_id>", Timestamp: time.Now().UTC()}
		}
		return b.stopServiceCommand(parts[1])

	case "/restart_service", "/restart":
		if len(parts) < 2 {
			return BotResponse{Text: "Использование: /restart_service <slug_or_id>", Timestamp: time.Now().UTC()}
		}
		return b.restartServiceCommand(parts[1])

	case "/bind":
		if len(parts) < 4 {
			return BotResponse{
				Text:      "Использование: /bind <slug> <port> <executable_path>\nПример: /bind docs 8081 ./bin/docs-server.exe",
				Timestamp: time.Now().UTC(),
			}
		}
		port, err := strconv.Atoi(parts[2])
		if err != nil {
			return BotResponse{Text: "❌ Недопустимый порт: " + parts[2], Timestamp: time.Now().UTC()}
		}
		return b.bindServiceExec(parts[1], port, parts[3])

	default:
		return BotResponse{
			Text:      fmt.Sprintf("Неизвестная команда '%s'. Нажмите /help для просмотра интерактивного меню.", cmd),
			Timestamp: time.Now().UTC(),
		}
	}
}

// HandleCallbackQuery processes inline button clicks.
func (b *AdminBot) HandleCallbackQuery(chatID int64, data string) BotResponse {
	if !b.IsAuthorized(chatID) {
		return BotResponse{Text: "⛔ Access Denied", Timestamp: time.Now().UTC()}
	}

	parts := strings.Split(data, ":")
	action := parts[0]
	targetID := ""
	if len(parts) > 1 {
		targetID = parts[1]
	}

	switch action {
	case "menu":
		switch targetID {
		case "services":
			return b.servicesResponse()
		case "ports":
			return b.portsResponse()
		case "health":
			return b.healthResponse()
		case "client":
			return b.clientDeliveryResponse()
		default:
			return b.helpResponse()
		}
	case "view":
		return b.serviceDetailResponse(targetID)
	case "logs":
		return b.logsResponse(targetID)
	case "start":
		return b.startServiceCommand(targetID)
	case "stop":
		return b.stopServiceCommand(targetID)
	case "restart":
		return b.restartServiceCommand(targetID)
	case "refresh":
		return b.servicesResponse()
	case "ports":
		return b.portsResponse()
	case "health":
		return b.healthResponse()
	default:
		return BotResponse{Text: "Действие завершено.", Timestamp: time.Now().UTC()}
	}
}

func (b *AdminBot) helpResponse() BotResponse {
	text := `🏛️ *WEBGATE CONTROL PLANE — ИНТЕРАКТИВНЫЙ HUD*
Мобильное управление сервисами, портами и доставкой клиентов

*Нажмите нужный раздел ниже для перехода:*`

	return BotResponse{
		Text: text,
		InlineKeyboard: [][]InlineButton{
			{
				{Text: "📊 Реестр сервисов", CallbackData: "menu:services"},
				{Text: "🔌 Карта портов", CallbackData: "menu:ports"},
			},
			{
				{Text: "⚡ Здоровье шлюза", CallbackData: "menu:health"},
				{Text: "📦 Получить клиент", CallbackData: "menu:client"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) servicesResponse() BotResponse {
	services := b.services.List()
	if len(services) == 0 {
		return BotResponse{
			Text: "Нет зарегистрированных сервисов в WebGate.",
			InlineKeyboard: [][]InlineButton{
				{{Text: "⬅️ Главное меню", CallbackData: "menu:root"}},
			},
			Timestamp: time.Now().UTC(),
		}
	}

	var sb strings.Builder
	sb.WriteString("📦 *РЕЕСТР СЕРВИСОВ И ЖИЗНЕННЫЙ ЦИКЛ:*\n")
	sb.WriteString("_Выберите сервис для детального управления и просмотра логов:_\n\n")

	var keyboard [][]InlineButton

	for i, s := range services {
		stateIcon := "🔴 STOPPED"
		if s.ProcessState == domain.ProcessStateRunning {
			stateIcon = fmt.Sprintf("🟢 RUNNING (PID %d)", s.ProcessPID)
		}

		portInfo := "N/A"
		if s.Port > 0 {
			portInfo = strconv.Itoa(s.Port)
		}

		sb.WriteString(fmt.Sprintf(
			"*%02d. %s* (`/svc/%s/`)\n"+
				"• Статус: *%s* | Порт: `%s`\n\n",
			i+1, s.Name, s.Slug,
			stateIcon, portInfo,
		))

		// Row of buttons for this service
		var row []InlineButton
		row = append(row, InlineButton{Text: fmt.Sprintf("⚙️ %s", s.Slug), CallbackData: "view:" + s.ID})
		if s.ProcessState == domain.ProcessStateRunning {
			row = append(row,
				InlineButton{Text: "⏹ Стоп", CallbackData: "stop:" + s.ID},
				InlineButton{Text: "🔄 Рестарт", CallbackData: "restart:" + s.ID},
			)
		} else {
			row = append(row,
				InlineButton{Text: "▶ Запуск", CallbackData: "start:" + s.ID},
			)
		}
		keyboard = append(keyboard, row)
	}

	keyboard = append(keyboard, []InlineButton{
		{Text: "🔄 Обновить статус", CallbackData: "menu:services"},
		{Text: "⬅️ Главное меню", CallbackData: "menu:root"},
	})

	return BotResponse{
		Text:           sb.String(),
		InlineKeyboard: keyboard,
		Timestamp:      time.Now().UTC(),
	}
}

func (b *AdminBot) serviceDetailResponse(serviceID string) BotResponse {
	svc := b.resolveService(serviceID)
	if svc == nil {
		return BotResponse{Text: "❌ Сервис не найден", Timestamp: time.Now().UTC()}
	}

	stateIcon := "🔴 ОСТАНОВЛЕН"
	if svc.ProcessState == domain.ProcessStateRunning {
		stateIcon = fmt.Sprintf("🟢 РАБОТАЕТ (PID %d)", svc.ProcessPID)
	}

	execPath := svc.ExecutablePath
	if execPath == "" {
		execPath = "Не задан"
	}

	text := fmt.Sprintf("📦 *СЕРВИС: %s*\n"+
		"• Идентификатор: `%s`\n"+
		"• Маршрут: `/svc/%s/`\n"+
		"• Состояние процесса: *%s*\n"+
		"• Выделенный порт: `%d`\n"+
		"• Upstream URL: `%s`\n"+
		"• Исполняемый файл: `%s`\n",
		svc.Name, svc.ID, svc.Slug, stateIcon, svc.Port, svc.UpstreamURL, execPath)

	var actionRow []InlineButton
	if svc.ProcessState == domain.ProcessStateRunning {
		actionRow = append(actionRow,
			InlineButton{Text: "⏹ Остановить", CallbackData: "stop:" + svc.ID},
			InlineButton{Text: "🔄 Перезапустить", CallbackData: "restart:" + svc.ID},
		)
	} else {
		actionRow = append(actionRow,
			InlineButton{Text: "▶ Запустить процесс", CallbackData: "start:" + svc.ID},
		)
	}

	keyboard := [][]InlineButton{
		actionRow,
		{
			{Text: "📋 Журнал логов", CallbackData: "logs:" + svc.ID},
			{Text: "🔌 Проверить порт", CallbackData: "menu:ports"},
		},
		{
			{Text: "⬅️ Назад к сервисам", CallbackData: "menu:services"},
			{Text: "🏠 Главное меню", CallbackData: "menu:root"},
		},
	}

	return BotResponse{
		Text:           text,
		InlineKeyboard: keyboard,
		Timestamp:      time.Now().UTC(),
	}
}

func (b *AdminBot) logsResponse(serviceID string) BotResponse {
	svc := b.resolveService(serviceID)
	if svc == nil {
		return BotResponse{Text: "❌ Сервис не найден: " + serviceID, Timestamp: time.Now().UTC()}
	}

	recent := b.procManager.GetRecentLogs(svc.ID, 12)
	logsFormatted := strings.Join(recent, "\n")

	text := fmt.Sprintf("📋 *ЖУРНАЛ ЛОГОВ: %s* (`%s`)\n```text\n%s\n```",
		svc.Name, svc.Slug, logsFormatted)

	keyboard := [][]InlineButton{
		{
			{Text: "🔄 Обновить логи", CallbackData: "logs:" + svc.ID},
			{Text: "⚙️ К сервису", CallbackData: "view:" + svc.ID},
		},
		{
			{Text: "⬅️ Список сервисов", CallbackData: "menu:services"},
		},
	}

	return BotResponse{
		Text:           text,
		InlineKeyboard: keyboard,
		Timestamp:      time.Now().UTC(),
	}
}

func (b *AdminBot) portsResponse() BotResponse {
	services := b.services.List()
	var sb strings.Builder
	sb.WriteString("🔌 *КАРТА ВЫДЕЛЕННЫХ ПОРТОВ WEBGATE:*\n\n")

	for _, s := range services {
		status := "🔴 STOPPED"
		if s.ProcessState == domain.ProcessStateRunning {
			status = "🟢 RUNNING"
		}
		sb.WriteString(fmt.Sprintf("• Порт *%d*: %s (`%s`) → %s\n", s.Port, s.Name, s.Slug, status))
	}

	return BotResponse{
		Text: sb.String(),
		InlineKeyboard: [][]InlineButton{
			{
				{Text: "📊 Управление сервисами", CallbackData: "menu:services"},
				{Text: "⬅️ Главное меню", CallbackData: "menu:root"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) healthResponse() BotResponse {
	services := b.services.List()
	runningCt := 0
	for _, s := range services {
		if s.ProcessState == domain.ProcessStateRunning {
			runningCt++
		}
	}

	text := fmt.Sprintf("⚡ *СТАТУС ЗДОРОВЬЯ WEBGATE*\n\n"+
		"• Состояние шлюза: 🟢 *HEALTHY*\n"+
		"• Всего сервисов: `%d`\n"+
		"• Активных процессов: `🟢 %d`\n"+
		"• Время сервера: `%s` UTC",
		len(services), runningCt, time.Now().UTC().Format("15:04:05 02.01.2006"))

	return BotResponse{
		Text: text,
		InlineKeyboard: [][]InlineButton{
			{
				{Text: "📊 Сервисы", CallbackData: "menu:services"},
				{Text: "⬅️ Главное меню", CallbackData: "menu:root"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) clientDeliveryResponse() BotResponse {
	text := `📦 *ДОСТАВКА КЛИЕНТСКИХ ПРИЛОЖЕНИЙ WEBGATE*
Выберите целевую платформу для скачивания криптографически подписанного клиента:`

	return BotResponse{
		Text: text,
		InlineKeyboard: [][]InlineButton{
			{
				{Text: "🪟 Windows (x86_64)", CallbackData: "delivery:windows:x86_64"},
				{Text: "🤖 Android (arm64)", CallbackData: "delivery:android:arm64"},
			},
			{
				{Text: "🐧 Linux (x86_64)", CallbackData: "delivery:linux:x86_64"},
				{Text: "🍎 macOS (arm64)", CallbackData: "delivery:macos:arm64"},
			},
			{
				{Text: "⬅️ Главное меню", CallbackData: "menu:root"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) confirmActionResponse(action, targetID, description string) BotResponse {
	text := fmt.Sprintf("⚠️ *ПОДТВЕРЖДЕНИЕ ДЕЙСТВИЯ*\n\nВы уверены, что хотите выполнить действие: *%s* для `%s`?",
		description, targetID)

	return BotResponse{
		Text: text,
		InlineKeyboard: [][]InlineButton{
			{
				{Text: "❌ Отмена", CallbackData: "view:" + targetID},
				{Text: "⚠️ ДА, ВЫПОЛНИТЬ", CallbackData: action + ":" + targetID},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) startServiceCommand(slugOrID string) BotResponse {
	svc := b.resolveService(slugOrID)
	if svc == nil {
		return BotResponse{Text: "❌ Сервис не найден: " + slugOrID, Timestamp: time.Now().UTC()}
	}

	inst, err := b.procManager.StartService(svc.ID)
	if err != nil {
		return BotResponse{
			Text:      fmt.Sprintf("❌ Ошибка запуска сервиса *%s*: %v", svc.Name, err),
			Timestamp: time.Now().UTC(),
		}
	}

	return BotResponse{
		Text: fmt.Sprintf("✅ *Сервис успешно запущен!*\n\n• Сервис: *%s* (`%s`)\n• Выделенный порт: `%d`\n• PID процесса: `%d`\n• Upstream: `%s`",
			svc.Name, svc.Slug, inst.Port, inst.PID, svc.UpstreamURL),
		InlineKeyboard: [][]InlineButton{
			{
				{Text: fmt.Sprintf("⏹ Остановить %s", svc.Slug), CallbackData: "stop:" + svc.ID},
				{Text: fmt.Sprintf("🔄 Перезапустить %s", svc.Slug), CallbackData: "restart:" + svc.ID},
			},
			{
				{Text: "📋 Логи процесса", CallbackData: "logs:" + svc.ID},
				{Text: "📊 Все сервисы", CallbackData: "menu:services"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) stopServiceCommand(slugOrID string) BotResponse {
	svc := b.resolveService(slugOrID)
	if svc == nil {
		return BotResponse{Text: "❌ Сервис не найден: " + slugOrID, Timestamp: time.Now().UTC()}
	}

	if err := b.procManager.StopService(svc.ID); err != nil {
		return BotResponse{
			Text:      fmt.Sprintf("❌ Ошибка остановки сервиса *%s*: %v", svc.Name, err),
			Timestamp: time.Now().UTC(),
		}
	}

	return BotResponse{
		Text: fmt.Sprintf("⏹ *Сервис остановлен!*\n\n• Сервис: *%s* (`%s`)\n• Статус: STOPPED\n• Порт `%d` освобожден",
			svc.Name, svc.Slug, svc.Port),
		InlineKeyboard: [][]InlineButton{
			{
				{Text: fmt.Sprintf("▶ Запустить %s", svc.Slug), CallbackData: "start:" + svc.ID},
				{Text: "📊 Все сервисы", CallbackData: "menu:services"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) restartServiceCommand(slugOrID string) BotResponse {
	svc := b.resolveService(slugOrID)
	if svc == nil {
		return BotResponse{Text: "❌ Сервис не найден: " + slugOrID, Timestamp: time.Now().UTC()}
	}

	inst, err := b.procManager.RestartService(svc.ID)
	if err != nil {
		return BotResponse{
			Text:      fmt.Sprintf("❌ Ошибка перезапуска сервиса *%s*: %v", svc.Name, err),
			Timestamp: time.Now().UTC(),
		}
	}

	return BotResponse{
		Text: fmt.Sprintf("🔄 *Сервис успешно перезапущен!*\n\n• Сервис: *%s* (`%s`)\n• Порт: `%d`\n• Новый PID: `%d`",
			svc.Name, svc.Slug, inst.Port, inst.PID),
		InlineKeyboard: [][]InlineButton{
			{
				{Text: fmt.Sprintf("⏹ Остановить %s", svc.Slug), CallbackData: "stop:" + svc.ID},
				{Text: "📋 Логи", CallbackData: "logs:" + svc.ID},
			},
			{
				{Text: "📊 Все сервисы", CallbackData: "menu:services"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) bindServiceExec(slugOrID string, port int, execPath string) BotResponse {
	svc := b.resolveService(slugOrID)
	if svc == nil {
		return BotResponse{Text: "❌ Сервис не найден: " + slugOrID, Timestamp: time.Now().UTC()}
	}

	svc.Port = port
	svc.ExecutablePath = execPath
	svc.UpstreamURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	svc.UpdatedAt = time.Now().UTC()

	return BotResponse{
		Text: fmt.Sprintf("✅ *Исполняемый файл и порт привязаны!*\n\n• Сервис: *%s* (`%s`)\n• Порт: `%d`\n• Файл: `%s`\n• Upstream: `%s`",
			svc.Name, svc.Slug, port, execPath, svc.UpstreamURL),
		InlineKeyboard: [][]InlineButton{
			{
				{Text: fmt.Sprintf("▶ Запустить %s", svc.Slug), CallbackData: "start:" + svc.ID},
			},
			{
				{Text: "📊 Все сервисы", CallbackData: "menu:services"},
			},
		},
		Timestamp: time.Now().UTC(),
	}
}

func (b *AdminBot) resolveService(query string) *domain.ProtectedService {
	if svc, err := b.services.ResolveBySlug(query); err == nil {
		return svc
	}
	if svc, err := b.services.GetByID(query); err == nil {
		return svc
	}
	return nil
}
