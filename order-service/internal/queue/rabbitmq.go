package queue

import (
	"context"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var Channel *amqp.Channel

func InitRabbitMQ(url string) {
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatal("Không thể kết nối RabbitMQ:", err)
	}

	Channel, err = conn.Channel()
	if err != nil {
		log.Fatal("Không thể mở Channel:", err)
	}

	// Tạo hàng đợi có tên "order_created"
	_, err = Channel.QueueDeclare("order_created", true, false, false, false, nil)
	if err != nil {
		log.Fatal("Lỗi khai báo Queue:", err)
	}
	log.Println("Đã kết nối RabbitMQ thành công!")
	// Tạo hàng đợi chứa tin nhắn Gửi Email
	_, err = Channel.QueueDeclare("notification_queue", false, false, false, false, nil)
	if err != nil {
		log.Fatal("Lỗi khai báo notification_queue:", err)
	}
}

// Hàm đẩy message vào hàng đợi
func PublishOrderCreated(body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return Channel.PublishWithContext(ctx, "", "order_created", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

// Thả hàm này vào file queue/rabbitmq.go của anh (ĐÃ SỬA LỖI & CHUẨN HÓA)
func PublishNotificationEvent(body []byte) {
	// Dùng context giới hạn thời gian 5 giây để tránh bị treo Server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// DÙNG BIẾN 'Channel' VÀ HÀM 'PublishWithContext' ĐÚNG CHUẨN CỦA ANH
	err := Channel.PublishWithContext(ctx,
		"",                   // exchange
		"notification_queue", // routing key (Tên hàng đợi cho việc gửi Email)
		false,                // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})

	if err != nil {
		log.Println("❌ Lỗi đẩy Notification vào RabbitMQ:", err)
	} else {
		log.Println("✅ Đã ném yêu cầu gửi Email vào RabbitMQ thành công!")
	}
}
