package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var Log *zap.Logger

// Khởi tạo Logger theo chuẩn Production (JSON Format)
func InitLogger() {
	var err error
	Log, err = zap.NewProduction()
	if err != nil {
		panic("Không thể khởi tạo Zap Logger: " + err.Error())
	}
}

// Middleware chặn mọi request để ghi log
func ZapLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Cho phép request chạy vào các Controller xử lý
		c.Next()

		// Đo thời gian xử lý xong
		cost := time.Since(start)

		// Ghi log ra Terminal bằng Zap
		Log.Info("API Request",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.Duration("latency", cost),
		)
	}
}
