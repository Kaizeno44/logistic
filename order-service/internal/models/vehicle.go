package models

import "gorm.io/gorm"

type Vehicle struct {
	gorm.Model
	DriverID uint `json:"driver_id"` // Thuộc về tài xế nào

	VehicleType  string  `json:"vehicle_type"`  // Xe máy, Xe bán tải, Xe tải 1 tấn...
	MaxWeight    float64 `json:"max_weight"`    // Tải trọng tối đa (kg)
	AllowedItems string  `json:"allowed_items"` // Các loại hàng được phép chở

	// Xe này có đang được tài xế chọn để chạy hôm nay không?
	IsActive bool `json:"is_active" gorm:"default:false"`
}
