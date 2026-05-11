package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"driver-service/internal/cache"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

// 1. Dữ liệu nhận từ Order Service (Khớp với Order struct)
type OrderMessage struct {
	ID               uint    `json:"id"`
	TrackingCode     string  `json:"tracking_code"`
	ItemType         string  `json:"item_type"`
	Weight           float64 `json:"weight"`
	PickupDistrictID uint    `json:"pickup_district_id"`
	PickupAddress    string  `json:"pickup_address"`
}

// 2. Định nghĩa cấu trúc Tài xế & Xe cộ để Database hiểu
type Driver struct {
	ID                uint   `gorm:"primaryKey"`
	CurrentDistrictID uint   `json:"current_district_id"`
	Status            string `json:"status"` // ONLINE, BUSY, OFFLINE
}

type Vehicle struct {
	ID           uint    `gorm:"primaryKey"`
	DriverID     uint    `json:"driver_id"`
	VehicleType  string  `json:"vehicle_type"`  // Xe máy, Xe tải...
	MaxWeight    float64 `json:"max_weight"`    // Tải trọng (kg)
	AllowedItems string  `json:"allowed_items"` // VD: "Thời trang, Mỹ phẩm"
	IsActive     bool    `json:"is_active"`
}

func StartOrderConsumer(rabbitURL string, db *gorm.DB) { // Thêm param db *gorm.DB
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatal("Driver Service: Lỗi kết nối RabbitMQ -", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Driver Service: Lỗi mở Channel -", err)
	}
	defer ch.Close()

	msgs, err := ch.Consume("order_created", "", false, false, false, false, nil)

	log.Println("🏍️ MATCHING ENGINE đang chờ đơn hàng mới...")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			var order OrderMessage
			json.Unmarshal(d.Body, &order)

			log.Printf("---------------------------------------------------")
			log.Printf("📦 TÌM KIẾM TÀI XẾ CHO ĐƠN: %s", order.TrackingCode)
			log.Printf("🔍 Yêu cầu: Nằm tại Quận ID %d | Nặng: %.1f kg | Loại: %s", order.PickupDistrictID, order.Weight, order.ItemType)

			var matchedDriver Driver

			// === THUẬT TOÁN MATCHING (CORE LOGIC) ===
			err := db.Table("drivers").
				Joins("JOIN vehicles ON vehicles.driver_id = drivers.id").
				Where("drivers.status = ?", "ONLINE"). // 1. Tài xế đang rảnh
				/*Where("drivers.current_district_id = ?", order.PickupDistrictID).*/ // 2. Cùng Quận lấy hàng
				Where("vehicles.is_active = ?", true).                                // 3. Xe đang sử dụng
				Where("vehicles.max_weight >= ?", order.Weight).                      // 4. Xe đủ tải trọng
				/*Where("vehicles.allowed_items LIKE ?", "%"+order.ItemType+"%").*/ // 5. Xe chở được loại hàng này
				First(&matchedDriver).Error

			if err == nil {
				// ✅ TÌM THẤY TÀI XẾ
				driverIDStr := fmt.Sprintf("TX-%d", matchedDriver.ID)
				log.Printf("🚀 THÀNH CÔNG: Đã chốt tài xế [%s] cho đơn hàng!", driverIDStr)

				// Khóa tài xế này trong Redis (Tránh nhận 2 đơn cùng lúc)
				cacheKey := "driver_status:" + driverIDStr
				cache.Rdb.Set(context.Background(), cacheKey, "BUSY", 30*time.Minute)

				// Cập nhật trạng thái tài xế và đơn hàng trong DB (Pending -> Matching)
				db.Model(&Driver{}).Where("id = ?", matchedDriver.ID).Update("status", "BUSY")
				db.Table("orders").Where("id = ?", order.ID).Update("status", "MATCHED")

				d.Ack(false) // Xóa tin nhắn khỏi RabbitMQ
			} else {
				// ❌ KHÔNG TÌM THẤY TÀI XẾ
				log.Printf("⚠️ THẤT BẠI: Không có tài xế nào thỏa mãn điều kiện lúc này.")

				// Ném đơn hàng lại vào Queue để 10 giây sau hệ thống quét lại
				time.Sleep(10 * time.Second)
				d.Nack(false, true)
			}
		}
	}()

	<-forever
}
