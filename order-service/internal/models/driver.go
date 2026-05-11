package models

import "gorm.io/gorm"

type Driver struct {
	gorm.Model
	// Cầu nối: Một User sẽ có một hồ sơ Driver
	UserID uint `json:"user_id" gorm:"uniqueIndex"`

	// Vị trí hiện tại của tài xế (để sau này matching theo khu vực)
	CurrentDistrictID uint `json:"current_district_id"`

	// Trạng thái: ONLINE (đang chờ đơn), BUSY (đang đi giao), OFFLINE
	Status string `json:"status" gorm:"default:'OFFLINE'"`
}
