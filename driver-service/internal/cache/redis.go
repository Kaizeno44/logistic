package cache

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client
var ctx = context.Background()

func InitRedis(redisURL string) {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     redisURL, // Lấy từ biến môi trường của Docker (redis-cache:6379)
		Password: "",       // Không dùng mật khẩu cho môi trường dev
		DB:       0,        // Dùng database mặc định
	})

	// Test kết nối
	_, err := Rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Driver Service: Lỗi kết nối Redis -", err)
	}
	log.Println("Đã kết nối Redis thành công!")
}

// Hàm giả lập: Tìm tài xế gần nhất
func FindNearestDriver(pickupAddress string) string {
	// Trong thực tế, anh sẽ dùng tính năng GEO của Redis (GEOSEARCH) để tìm kiếm tọa độ.
	// Ở đây ta giả lập Redis trả về ID của một tài xế đang rảnh ở gần đó.

	driverID := "TX-999"

	// Lưu trạng thái tài xế vào Redis (Khóa tài xế này lại trong 10 phút để đi đón khách)
	cacheKey := fmt.Sprintf("driver_status:%s", driverID)
	err := Rdb.Set(ctx, cacheKey, "BUSY", 10*60*1000000000).Err()
	if err != nil {
		log.Println("Lỗi lưu trạng thái Redis:", err)
		return ""
	}

	return driverID
}
