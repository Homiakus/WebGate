package config

// DurableServerConfig contains only WebGate-owned control-plane metadata.
type DurableServerConfig struct {
	ServerName            string           `json:"server_name"`
	ListenAddr            string           `json:"listen_addr"`
	AdminAddr             string           `json:"admin_addr"`
	ProxyTimeoutSecs      int              `json:"proxy_timeout_seconds"`
	TelegramBotToken      string           `json:"telegram_bot_token,omitempty"`
	TelegramBotEnabled    bool             `json:"telegram_bot_enabled"`
	TelegramAdminChatIDs  []int64          `json:"telegram_admin_chat_ids,omitempty"`
	TelegramAPIEndpoint   string           `json:"telegram_api_endpoint,omitempty"`
	TelegramMaxFileSizeMB int              `json:"telegram_max_file_size_mb,omitempty"`
	RelayNodes            []RelayNodeEntry `json:"relay_nodes,omitempty"`
}

func DurableSnapshot(cfg *ServerConfig) *DurableServerConfig {
	if cfg == nil {
		return nil
	}
	return &DurableServerConfig{
		ServerName:            cfg.ServerName,
		ListenAddr:            cfg.ListenAddr,
		AdminAddr:             cfg.AdminAddr,
		ProxyTimeoutSecs:      cfg.ProxyTimeoutSecs,
		TelegramBotToken:      cfg.TelegramBotToken,
		TelegramBotEnabled:    cfg.TelegramBotEnabled,
		TelegramAdminChatIDs:  append([]int64(nil), cfg.TelegramAdminChatIDs...),
		TelegramAPIEndpoint:   cfg.TelegramAPIEndpoint,
		TelegramMaxFileSizeMB: cfg.TelegramMaxFileSizeMB,
		RelayNodes:            append([]RelayNodeEntry(nil), cfg.RelayNodes...),
	}
}

func ApplyDurableSnapshot(cfg *ServerConfig, durable *DurableServerConfig) {
	if cfg == nil || durable == nil {
		return
	}
	cfg.ServerName = durable.ServerName
	cfg.ListenAddr = durable.ListenAddr
	cfg.AdminAddr = durable.AdminAddr
	cfg.ProxyTimeoutSecs = durable.ProxyTimeoutSecs
	cfg.TelegramBotToken = durable.TelegramBotToken
	cfg.TelegramBotEnabled = durable.TelegramBotEnabled
	cfg.TelegramAdminChatIDs = append([]int64(nil), durable.TelegramAdminChatIDs...)
	cfg.TelegramAPIEndpoint = durable.TelegramAPIEndpoint
	cfg.TelegramMaxFileSizeMB = durable.TelegramMaxFileSizeMB
	cfg.RelayNodes = append([]RelayNodeEntry(nil), durable.RelayNodes...)
}

func CloneServerConfig(cfg *ServerConfig) *ServerConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.TelegramAdminChatIDs = append([]int64(nil), cfg.TelegramAdminChatIDs...)
	clone.Services = append([]ProtectedServiceEntry(nil), cfg.Services...)
	for i := range clone.Services {
		clone.Services[i].ExecArgs = append([]string(nil), cfg.Services[i].ExecArgs...)
	}
	clone.RelayNodes = append([]RelayNodeEntry(nil), cfg.RelayNodes...)
	return &clone
}

func RedactedCopy(cfg *ServerConfig) *ServerConfig {
	clone := CloneServerConfig(cfg)
	if clone != nil {
		clone.TelegramBotToken = ""
	}
	return clone
}
