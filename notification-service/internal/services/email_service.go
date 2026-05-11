package services

import (
	"log"
	"net/smtp"

	"notification-service/internal/config"
)

func SendRealEmail(cfg *config.Config, targetEmail, otpCode string) {
	to := []string{targetEmail}

	message := []byte("From: " + cfg.SMTPEmail + "\r\n" +
		"To: " + targetEmail + "\r\n" +
		"Subject: Ma OTP khoi phuc mat khau Logistics\r\n\r\n" +
		"Ma OTP cua ban la: " + otpCode + ". Ma nay se het han sau 5 phut.")

	auth := smtp.PlainAuth("", cfg.SMTPEmail, cfg.SMTPPassword, cfg.SMTPHost)
	err := smtp.SendMail(cfg.SMTPHost+":"+cfg.SMTPPort, auth, cfg.SMTPEmail, to, message)
	if err != nil {
		log.Println("❌ Lỗi gửi email:", err)
	} else {
		log.Println("✅ Đã gửi Email thành công tới:", targetEmail)
	}
}
