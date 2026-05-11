package middlewares

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Khóa bí mật dùng để ký token (Trong thực tế nên để ở file .env)
var secretKey = []byte("uth_logistics_secret_key_2026")

// 1. Hàm tạo JWT Token (Dùng cho API Login)
func GenerateToken(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"role":    "customer",
		"exp":     time.Now().Add(time.Hour * 2).Unix(), // Token hết hạn sau 2 tiếng
	})
	return token.SignedString(secretKey)
}

// 2. Middleware chặn các request không có Token hợp lệ
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Lấy token từ header "Authorization: Bearer <token>"
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Thiếu token xác thực!"})
			return
		}

		// Cắt bỏ chữ "Bearer " để lấy đúng chuỗi token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Giải mã và kiểm tra tính hợp lệ của token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("phương thức ký không hợp lệ")
			}
			return secretKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token không hợp lệ hoặc đã hết hạn!"})
			return
		}

		// Lấy thông tin user_id từ token và lưu vào context để các API sau dùng
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("user_id", claims["user_id"])
		}

		c.Next() // Cho phép đi tiếp vào API tạo đơn hàng
	}
}
