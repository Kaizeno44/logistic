package queue

import (
	"encoding/json"
	"log"

	"github.com/streadway/amqp"

	"notification-service/internal/config"
	"notification-service/internal/services"
)

func StartRabbitMQConsumer(cfg *config.Config) {
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatal("Không thể kết nối RabbitMQ:", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Không thể mở Channel:", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"notification_queue", // Tên hàng đợi
		false,                // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	log.Println("🎧 [Notification Service] Đang đeo tai nghe, chờ lệnh từ RabbitMQ...")

	for d := range msgs {
		var data map[string]string

		if err := json.Unmarshal(d.Body, &data); err != nil {
			log.Println("Lỗi parse JSON:", err)
			continue
		}

		log.Printf("📩 Nhận được lệnh gửi Email tới: %s", data["email"])

		if data["type"] == "FORGOT_PASSWORD" {
			services.SendRealEmail(cfg, data["email"], data["otp"])
		}
	}
}
