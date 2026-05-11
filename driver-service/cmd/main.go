package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"driver-service/internal/cache"
	"driver-service/internal/workers"

	// ĐÃ MỞ KHÓA IMPORT HÀNG THẬT
	"driver-service/internal/middlewares"
	"driver-service/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// CHỈ ĐỊNH RÕ PORT CỦA FRONTEND (Ở đây là cổng 80 của Web UI)
		// Nếu lúc test Frontend chạy port 5500 (Live Server) thì anh đổi thành http://localhost:5500 nhé
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:80") 
		
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	redisURL := os.Getenv("REDIS_HOST")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}
	dbDSN := os.Getenv("DB_DSN")
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8002"
	}


	// 1. Kết nối DB thông minh có Retry
	var err error
	for i := 1; i <= 10; i++ {
		DB, err = gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
		if err == nil {
			log.Println("✅ Đã kết nối PostgreSQL thành công!")
			break
		}
		log.Printf("⏳ Database chưa sẵn sàng (Lần thử %d/10). Chờ 3s...\n", i)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal("❌ Bỏ cuộc! Không thể kết nối Database:", err)
	}

	// (Làm tương tự với RabbitMQ nếu có kết nối trực tiếp ở main)

	err = DB.AutoMigrate(
		&models.Order{},
		&models.Driver{},
		&models.Vehicle{},
	)
	if err != nil {
		log.Fatal("Lỗi khi AutoMigrate Database:", err)
	}
	log.Println("Đã đồng bộ cấu trúc 3 bảng: Orders, Drivers, Vehicles thành công!")
	cache.InitRedis(redisURL)
	go workers.StartOrderConsumer(rabbitURL, DB)

	r := gin.Default()
	r.Use(CORSMiddleware())

	// NẾU CÓ MIDDLEWARE LOGGER THÌ BẬT LÊN LUÔN
	// middlewares.InitLogger()
	// r.Use(middlewares.ZapLogger())

	r.GET("/api/drivers/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "Driver Service Matching Engine is running!",
		})
	})

	// ==========================================
	// BẢO VỆ BẰNG JWT THẬT TỪ THƯ MỤC MIDDLEWARES
	// ==========================================
	protected := r.Group("/api")

	// DÙNG HÀNG THẬT: Kiểm tra token y hệt như bên Order Service!
	protected.Use(middlewares.JWTAuth())

	// 1. API CẬP NHẬT TRẠNG THÁI ĐƠN HÀNG (Hoàn thành cuốc)
	protected.PUT("/orders/:tracking_code/status", func(c *gin.Context) {
		trackingCode := c.Param("tracking_code")
		var input struct {
			Status string `json:"status" binding:"required"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trạng thái không hợp lệ"})
			return
		}

		result := DB.Table("orders").Where("tracking_code = ?", trackingCode).Update("status", input.Status)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể cập nhật trạng thái"})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy đơn hàng này"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật trạng thái đơn hàng thành " + input.Status})
	})

	// 2. THUẬT TOÁN MATCHING CỐT LÕI
	// 2. THUẬT TOÁN MATCHING CỐT LÕI (CHUẨN NGHIỆP VỤ)
	// 2. THUẬT TOÁN MATCHING CỐT LÕI (CHUẨN NGHIỆP VỤ - ĐÃ FIX LỖI GORM)
	protected.GET("/driver/current-order", func(c *gin.Context) {
		userIDStr := fmt.Sprintf("%v", c.MustGet("user_id"))
		userID, _ := strconv.Atoi(userIDStr)

		// 1. Kiểm tra Tài xế
		var driver models.Driver
		if err := DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Tài khoản chưa có hồ sơ Tài xế"})
			return
		}
		// NẾU TÀI XẾ ĐANG OFFLINE -> DỪNG TÌM ĐƠN VÀ BÁO VỀ GIAO DIỆN
		if driver.Status == "OFFLINE" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Tài xế đang tắt ứng dụng"})
			return
		}
		// 2. Kiểm tra Xe đang hoạt động
		var vehicle models.Vehicle
		if err := DB.Where("driver_id = ? AND is_active = ?", driver.ID, true).First(&vehicle).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Bạn chưa có xe nào đang hoạt động"})
			return
		}

		// 3. TÌM ĐƠN HÀNG (DÙNG ĐÚNG STRUCT models.Order ĐỂ KHÔNG BỊ LỖI SQL)
		var order models.Order
		err := DB.Where("status IN ? AND weight <= ?", []string{"PENDING", "MATCHED"}, vehicle.MaxWeight).
			Order("weight desc, created_at asc").
			First(&order).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Hệ thống đang tìm đơn..."})
			return
		}

		// 4. Khóa đơn
		// 4. Khóa đơn an toàn (Chống Race Condition)
		if order.Status == "PENDING" {
			// Cập nhật với điều kiện nghiêm ngặt: Chỉ update nếu nó THỰC SỰ vẫn đang PENDING
			result := DB.Model(&models.Order{}).
				Where("id = ? AND status = ?", order.ID, "PENDING").
				Update("status", "MATCHED")

			if result.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống khi nhận đơn"})
				return
			}

			// Nếu RowsAffected == 0, nghĩa là trong tích tắc vừa rồi có người khác đã nhận mất
			if result.RowsAffected == 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "Rất tiếc, đơn hàng vừa bị tài xế khác nhận mất!"})
				return
			}

			order.Status = "MATCHED"
		}
		c.JSON(http.StatusOK, order)
	})
	// ==========================================
	// API MỚI: BẬT/TẮT TRẠNG THÁI NHẬN ĐƠN CỦA TÀI XẾ
	// ==========================================
	protected.PUT("/drivers/status", func(c *gin.Context) {
		userIDStr := fmt.Sprintf("%v", c.MustGet("user_id"))
		userID, _ := strconv.Atoi(userIDStr)

		var input struct {
			Status string `json:"status" binding:"required"` // ONLINE hoặc OFFLINE
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
			return
		}

		// Cập nhật trạng thái trong Database
		result := DB.Model(&models.Driver{}).Where("user_id = ?", userID).Update("status", input.Status)
		if result.Error != nil || result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy hồ sơ tài xế"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Đã đổi trạng thái thành " + input.Status})
	})
	// ==========================================
	// API MỚI: ĐĂNG KÝ PHƯƠNG TIỆN CHẠY TRONG NGÀY
	// ==========================================
	protected.POST("/drivers/register-vehicle", func(c *gin.Context) {
		// Lấy user_id từ Token (đang là chuỗi string)
		userIDStr := fmt.Sprintf("%v", c.MustGet("user_id"))

		// ĐÂY LÀ CHÌA KHÓA: Ép kiểu chuỗi thành số nguyên (int)
		userID, _ := strconv.Atoi(userIDStr)

		var input struct {
			VehicleType string  `json:"vehicle_type" binding:"required"`
			MaxWeight   float64 `json:"max_weight" binding:"required"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
			return
		}

		// 1. TÌM HOẶC TẠO MỚI HỒ SƠ TÀI XẾ (DRIVER) BẰNG GORM
		var driver models.Driver
		if err := DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
			// Không tìm thấy -> Tạo mới
			driver = models.Driver{
				UserID: uint(userID),
				Status: "ONLINE",
			}
			if err := DB.Create(&driver).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi khi tạo hồ sơ Tài xế"})
				return
			}
		}

		DB.Model(&models.Vehicle{}).Where("driver_id = ?", driver.ID).Update("is_active", false)

		// 3. TÌM VÀ CẬP NHẬT (UPSERT)
		// Tìm xem tài xế này đã từng đăng ký loại xe này trong quá khứ chưa?
		var vehicle models.Vehicle
		err := DB.Where("driver_id = ? AND vehicle_type = ?", driver.ID, input.VehicleType).First(&vehicle).Error

		if err != nil {
			// TRƯỜNG HỢP A: Lần đầu tiên chạy loại xe này -> TẠO DÒNG MỚI
			newVehicle := models.Vehicle{
				DriverID:    driver.ID,
				VehicleType: input.VehicleType,
				MaxWeight:   input.MaxWeight,
				IsActive:    true,
			}
			if err := DB.Create(&newVehicle).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi tạo xe mới"})
				return
			}
		} else {
			// TRƯỜNG HỢP B: Đã từng chạy xe này rồi -> BẬT NÓ LÊN LẠI (UPDATE)
			if err := DB.Model(&vehicle).Updates(map[string]interface{}{
				"is_active":  true,
				"max_weight": input.MaxWeight, // Đề phòng tải trọng có thay đổi
			}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi cập nhật xe cũ"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật phương tiện thành công!"})

	})
	log.Println("🚀 Driver Service chạy port:" + port)
	r.Run(":" + port)
}
