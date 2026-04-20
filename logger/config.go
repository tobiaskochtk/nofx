package logger

import "github.com/sirupsen/logrus"

// Config is the logger configuration (simplified version)
type Config struct {
	Level    string          `json:"level"` // Log level: debug, info, warn, error (default: info)
	Telegram *TelegramConfig `json:"telegram,omitempty"`
}

// SetDefaults sets default values
func (c *Config) SetDefaults() {
	if c.Level == "" {
		c.Level = "info"
	}
}

// TelegramConfig controls optional Telegram log forwarding.
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   int64  `json:"chat_id"`
	MinLevel string `json:"min_level"` // debug, info, warn, error, fatal, panic
}

// GetLogrusLevels maps MinLevel to logrus hook levels.
func (c *TelegramConfig) GetLogrusLevels() []logrus.Level {
	if c == nil {
		return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
	}

	switch c.MinLevel {
	case "debug":
		return logrus.AllLevels
	case "info":
		return []logrus.Level{
			logrus.InfoLevel, logrus.WarnLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel,
		}
	case "warn":
		return []logrus.Level{
			logrus.WarnLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel,
		}
	case "fatal":
		return []logrus.Level{logrus.FatalLevel, logrus.PanicLevel}
	case "panic":
		return []logrus.Level{logrus.PanicLevel}
	case "error":
		fallthrough
	default:
		return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
	}
}
