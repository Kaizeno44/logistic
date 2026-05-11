package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"notification-service/internal/config"
	"notification-service/internal/handlers"
	"notification-service/internal/queue"
)

func main() {
	// 1. Tải cấu hình
	cfg := config.LoadConfig()

	// 2. Khởi chạy Worker ở riêng một nhánh (Goroutine)
	go queue.StartRabbitMQConsumer(cfg)

	// 3. Thiết lập Web Server (API HTTP)
	r := gin.Default()
	r.GET("/api/notifications/health", handlers.HealthCheck)

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	// 4. Chạy HTTP web server trong một Goroutine để nó không block luồng xử lý tín hiệu
	go func() {
		log.Printf("🚀 HTTP Server đang khởi chạy trên port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Lỗi khởi chạy HTTP Server: %s\n", err)
		}
	}()

	// 5. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Nhận được tín hiệu dừng server, đang dọn dẹp...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("❌ Lỗi khi tắt HTTP Server:", err)
	}

	log.Println("✅ Notification Service đã dừng hoàn toàn!")
}
