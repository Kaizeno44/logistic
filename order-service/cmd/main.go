package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"

	//"net/smtp"
	"os"
	"time"

	"order-service/internal/middlewares"
	"order-service/internal/models"
	"order-service/internal/queue"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Struct ánh xạ với bảng districts trong Database
type District struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name"`
}

// Struct ánh xạ với bảng wards trong Database
type Ward struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	Name       string `json:"name"`
	DistrictID uint   `json:"district_id"`
}

func main() {
	// 1. Lấy biến môi trường từ docker-compose
	dbDSN := os.Getenv("DB_DSN")
	rabbitURL := os.Getenv("RABBITMQ_URL")
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8001"
	}

	// 2. Kết nối PostgreSQL
	// XÓA DÒNG NÀY: time.Sleep(5 * time.Second)

	// 2. Kết nối PostgreSQL thông minh có vòng lặp Retry
	var db *gorm.DB
	var err error
	for i := 1; i <= 10; i++ {
		db, err = gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
		if err == nil {
			fmt.Println("✅ Đã kết nối PostgreSQL thành công!")
			break
		}
		log.Printf("⏳ Database chưa sẵn sàng (Lần thử %d/10). Chờ 3s...\n", i)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal("❌ Lỗi kết nối Database:", err)
	}
	db.AutoMigrate(&models.User{}, &models.Order{}, &models.Driver{}, &models.Vehicle{})
	fmt.Println("Đã kết nối và Migrate PostgreSQL thành công!")

	// 3. Kết nối RabbitMQ
	queue.InitRabbitMQ(rabbitURL)

	// 4. Kết nối Redis (Dùng cho OTP Quên mật khẩu)
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisHost})
	ctx := context.Background()

	// 5. Khởi tạo REST API
	middlewares.InitLogger()
	defer middlewares.Log.Sync()

	r := gin.New()
	// Cấu hình CORS mở rộng cho phép nhận Token
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(corsConfig))
	r.Use(middlewares.ZapLogger(), gin.Recovery())

	// ==========================================
	// 1. API ĐĂNG KÝ
	// ==========================================
	r.POST("/api/register", func(c *gin.Context) {
		var input struct {
			Username string `json:"username" binding:"required"`
			Email    string `json:"email" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu thông tin đăng ký"})
			return
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		newUser := models.User{
			Username: input.Username,
			Email:    input.Email,
			Password: string(hashedPassword),
			Role:     "customer",
		}

		if err := db.Create(&newUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tên đăng nhập hoặc Email đã tồn tại!"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"message": "Tạo tài khoản thành công!"})
	})

	// ==========================================
	// 2. API ĐĂNG NHẬP
	// ==========================================
	r.POST("/api/login", func(c *gin.Context) {
		var input struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
			return
		}

		var user models.User
		if err := db.Where("username = ?", input.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sai tài khoản hoặc mật khẩu!"})
			return
		}

		err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sai tài khoản hoặc mật khẩu!"})
			return
		}

		userIDStr := fmt.Sprintf("%d", user.ID)
		token, err := middlewares.GenerateToken(userIDStr, user.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi tạo token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Đăng nhập thành công!",
			"token":   token,
			"role":    user.Role,
		})
	})

	// ==========================================
	// API: ĐĂNG KÝ TÀI XẾ (ĐÃ FIX LỖI BĂM MẬT KHẨU)
	// ==========================================
	r.POST("/api/auth/register-driver", func(c *gin.Context) {
		var input struct {
			Username string `json:"username" binding:"required"`
			Email    string `json:"email" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập đủ thông tin!"})
			return
		}

		// ĐÂY LÀ CHỖ QUAN TRỌNG NHẤT: Băm mật khẩu ra mã $2a$10...
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi mã hóa mật khẩu"})
			return
		}

		// Tạo User mới nhưng nhét cái hashedPassword vào thay vì cái mật khẩu thô
		user := models.User{
			Username: input.Username,
			Email:    input.Email,
			Password: string(hashedPassword), // ĐÃ ĐƯỢC BĂM!
			Role:     "driver",
		}

		// Lưu vào DB
		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email hoặc Tên đăng nhập đã tồn tại!"})
			return
		}

		// Tạo Token ngay sau khi đăng ký
		tokenString, err := middlewares.GenerateToken(fmt.Sprintf("%d", user.ID), user.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi tạo token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Chào mừng bạn gia nhập đội ngũ Tài xế!",
			"token":   tokenString,
		})
	})

	// ==========================================
	// ==========================================
	// 3. API QUÊN MẬT KHẨU (Gửi OTP qua RabbitMQ)
	// ==========================================
	r.POST("/api/forgot-password", func(c *gin.Context) {
		var input struct {
			Username string `json:"username"`
		}
		c.ShouldBindJSON(&input)

		var user models.User
		if err := db.Where("username = ?", input.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy tài khoản!"})
			return
		}

		// Tạo OTP và lưu vào Redis để lát nữa xác thực
		otp := fmt.Sprintf("%06d", rand.Intn(1000000))
		redisKey := "otp:" + input.Username
		rdb.Set(ctx, redisKey, otp, 5*time.Minute)

		// ----------------------------------------------------
		// ĐÃ SỬA THÀNH MICROSERVICES: KHÔNG GỬI EMAIL TRỰC TIẾP
		// ----------------------------------------------------
		// Gói thông tin cần gửi vào một cái hộp (JSON)
		msg := map[string]string{
			"email": user.Email,
			"otp":   otp,
			"type":  "FORGOT_PASSWORD",
		}
		msgJSON, _ := json.Marshal(msg)

		// Hét lên RabbitMQ: "Ê Notification Service, gửi giùm cái email này!"
		// (Giả định anh đã có hàm Publish trong thư mục queue)
		queue.PublishNotificationEvent(msgJSON)

		c.JSON(http.StatusOK, gin.H{"message": "Mã OTP đã được đưa vào hàng đợi xử lý! Vui lòng kiểm tra hộp thư."})
	})

	// ==========================================
	// 4. API ĐẶT LẠI MẬT KHẨU (Xác thực OTP)
	// ==========================================
	r.POST("/api/reset-password", func(c *gin.Context) {
		var input struct {
			Username    string `json:"username"`
			OTP         string `json:"otp"`
			NewPassword string `json:"new_password"`
		}
		c.ShouldBindJSON(&input)

		redisKey := "otp:" + input.Username
		savedOTP, err := rdb.Get(ctx, redisKey).Result()

		if err == redis.Nil || savedOTP != input.OTP {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mã OTP không chính xác hoặc đã hết hạn!"})
			return
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		db.Model(&models.User{}).Where("username = ?", input.Username).Update("password", string(hashedPassword))
		rdb.Del(ctx, redisKey)

		c.JSON(http.StatusOK, gin.H{"message": "Đổi mật khẩu thành công!"})
	})

	// ==========================================
	// 5. CÁC API CẦN XÁC THỰC (Bảo vệ bằng JWT)
	// ==========================================
	protected := r.Group("/api")
	protected.Use(middlewares.JWTAuth())
	{
		// API 5.1: Tạo đơn hàng mới
		protected.POST("/orders", func(c *gin.Context) {
			userID := c.MustGet("user_id").(string)

			var newOrder models.Order
			if err := c.ShouldBindJSON(&newOrder); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			newOrder.CustomerID = userID
			newOrder.Status = "PENDING"
			// Sửa lại dòng tạo mã Tracking ở API 5.1:
			newOrder.TrackingCode = fmt.Sprintf("VN%d", time.Now().UnixNano()/1000000)

			// ==========================================
			// LOGIC TÍNH CƯỚC PHÍ TỰ ĐỘNG
			// ==========================================
			if newOrder.Weight <= 50 {
				newOrder.TotalFee = 35000 // Xe máy: Đồng giá 35k
			} else if newOrder.Weight <= 500 {
				newOrder.TotalFee = 150000 // Xe bán tải: Đồng giá 150k
			} else {
				newOrder.TotalFee = 350000 // Xe tải 1 Tấn: Đồng giá 350k
			}

			if err := db.Create(&newOrder).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi lưu DB"})
				return
			}

			orderJSON, _ := json.Marshal(newOrder)
			queue.PublishOrderCreated(orderJSON)

			c.JSON(http.StatusCreated, gin.H{
				"message": "Đã tạo đơn thành công!",
				"data":    newOrder,
			})
		})

		// API 5.2: LẤY LỊCH SỬ ĐƠN HÀNG (MỚI THÊM)
		// API 5.2: LẤY LỊCH SỬ ĐƠN HÀNG (Đã fix lỗi an toàn kiểu dữ liệu)
		protected.GET("/orders/history", func(c *gin.Context) {
			// Ép kiểu an toàn bằng fmt.Sprintf để không bị panic khi parse JWT
			userID := fmt.Sprintf("%v", c.MustGet("user_id"))

			var orders []models.Order
			// Lấy danh sách, xếp mới nhất lên đầu
			if err := db.Where("customer_id = ?", userID).Order("created_at desc").Find(&orders).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lấy lịch sử đơn hàng"})
				return
			}
			c.JSON(http.StatusOK, orders)
		})

		// ==========================================
		// API 5.3: ADMIN DASHBOARD (Lấy số liệu tổng quát)
		// ==========================================
		protected.GET("/admin/dashboard", middlewares.AdminAuth(), func(c *gin.Context) {
			var totalOrders int64
			var pendingOrders int64
			var completedOrders int64
			var recentOrders []models.Order

			// 1. Tính tổng số đơn hàng trong hệ thống
			db.Model(&models.Order{}).Count(&totalOrders)

			// 2. Đếm số đơn đang chờ xử lý (RabbitMQ chưa kịp ghép hoặc đang kẹt)
			db.Model(&models.Order{}).Where("status = ?", "PENDING").Count(&pendingOrders)

			// 3. Đếm số đơn đã hoàn thành
			db.Model(&models.Order{}).Where("status = ?", "COMPLETED").Count(&completedOrders)

			// 4. Lấy 5 đơn hàng mới nhất để đưa vào bảng luồng điều phối
			db.Order("created_at desc").Limit(5).Find(&recentOrders)

			// LƯU Ý: Vì bảng Order hiện tại của anh chưa lưu cột Cước Phí,
			// Nên em sẽ tính Doanh thu ước tính (Trung bình 30.000đ / đơn hoàn thành).
			// Nếu muốn chuẩn, anh cần lưu TotalFee vào DB lúc đặt đơn nhé!
			estimatedRevenue := completedOrders * 30000

			c.JSON(http.StatusOK, gin.H{
				"total_orders":     totalOrders,
				"pending_orders":   pendingOrders,
				"completed_orders": completedOrders,
				"revenue":          estimatedRevenue,
				"recent_orders":    recentOrders,
			})
		})
		// ==========================================

		// ==========================================
		// API 5.4: ADMIN QUẢN LÝ ĐƠN HÀNG (Lấy tất cả đơn)
		// ==========================================
		protected.GET("/admin/orders", middlewares.AdminAuth(), func(c *gin.Context) {
			var orders []models.Order
			if err := db.Order("created_at desc").Find(&orders).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lấy danh sách đơn hàng"})
				return
			}
			c.JSON(http.StatusOK, orders)
		})

		// ==========================================
		// API 5.5: ADMIN QUẢN LÝ TÀI XẾ
		// ==========================================
		protected.GET("/admin/drivers", middlewares.AdminAuth(), func(c *gin.Context) {
			type DriverResponse struct {
				ID       uint   `json:"id"`
				Username string `json:"username"`
				Email    string `json:"email"`
				Status   string `json:"status"`
			}
			var results []DriverResponse
			err := db.Table("users").
				Select("users.id, users.username, users.email, COALESCE(drivers.status, 'OFFLINE') as status").
				Joins("LEFT JOIN drivers ON drivers.user_id = users.id").
				Where("users.role = ?", "driver").
				Scan(&results).Error

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lấy danh sách tài xế"})
				return
			}
			c.JSON(http.StatusOK, results)
		})

		// ==========================================
		// API 5.6: ADMIN RABBITMQ STATS PROXY
		// ==========================================
		protected.GET("/admin/rabbitmq/stats", middlewares.AdminAuth(), func(c *gin.Context) {
			rabbitURLParsed, err := url.Parse(os.Getenv("RABBITMQ_URL"))
			rabbitHost := "rabbitmq-broker"
			if err == nil && rabbitURLParsed.Hostname() != "" {
				rabbitHost = rabbitURLParsed.Hostname()
			}

			client := &http.Client{Timeout: 5 * time.Second}
			req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:15672/api/queues", rabbitHost), nil)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi tạo request RabbitMQ"})
				return
			}
			req.SetBasicAuth("guest", "guest")

			resp, err := client.Do(req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi kết nối đến RabbitMQ Management API: " + err.Error()})
				return
			}
			defer resp.Body.Close()

			var stats interface{}
			if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi giải mã dữ liệu RabbitMQ"})
				return
			}

			c.JSON(http.StatusOK, stats)
		})
		// ==========================================
		// API: ADMIN KHÓA TÀI KHOẢN TÀI XẾ
		// ==========================================
		protected.PUT("/admin/drivers/:id/lock", middlewares.AdminAuth(), func(c *gin.Context) {
			driverID := c.Param("id")

			// Đổi Role của tài xế thành "locked" để tước quyền đăng nhập
			result := db.Model(&models.User{}).
				Where("id = ? AND role = ?", driverID, "driver").
				Update("role", "locked")

			if result.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống khi khóa tài khoản!"})
				return
			}

			if result.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy tài xế này hoặc đã bị khóa!"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Đã khóa tài xế thành công!"})
		})

		// ==========================================
		// API 5.6: XEM CHI TIẾT 1 ĐƠN HÀNG (MỚI THÊM)
		// ==========================================
		protected.GET("/orders/:tracking_code", func(c *gin.Context) {
			trackingCode := c.Param("tracking_code")
			userID := fmt.Sprintf("%v", c.MustGet("user_id"))

			var order models.Order
			// Phải check thêm customer_id = userID để khách này không xem lén được đơn của khách khác
			if err := db.Where("tracking_code = ? AND customer_id = ?", trackingCode, userID).First(&order).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy đơn hàng hoặc bạn không có quyền xem!"})
				return
			}

			c.JSON(http.StatusOK, order)
		})
	}

	// 1. API lấy danh sách TẤT CẢ các Quận/Huyện (Không cần JWT)
	r.GET("/api/districts", func(c *gin.Context) {
		var districts []District
		if err := db.Find(&districts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi lấy dữ liệu Quận"})
			return
		}
		c.JSON(http.StatusOK, districts)
	})

	// 2. API lấy danh sách Phường/Xã THEO ID của Quận (Không cần JWT)
	r.GET("/api/districts/:id/wards", func(c *gin.Context) {
		districtID := c.Param("id")
		var wards []Ward
		if err := db.Where("district_id = ?", districtID).Find(&wards).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi lấy dữ liệu Phường"})
			return
		}
		c.JSON(http.StatusOK, wards)
	})

	r.Run(":" + port)
}
