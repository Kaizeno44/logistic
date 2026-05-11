package config

import (
	"os"
)

type Config struct {
	SMTPHost     string
	SMTPPort     string
	SMTPEmail    string
	SMTPPassword string
	RabbitMQURL  string
	ServerPort   string
}

func LoadConfig() *Config {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@rabbitmq:5672/"
	}

	smtpEmail := os.Getenv("SMTP_EMAIL")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	serverPort := os.Getenv("PORT")
	if serverPort == "" {
		serverPort = "8003"
	}

	return &Config{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     "587",
		SMTPEmail:    smtpEmail,
		SMTPPassword: smtpPassword,
		RabbitMQURL:  rabbitURL,
		ServerPort:   serverPort,
	}
}
