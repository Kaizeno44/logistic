package models

import "gorm.io/gorm"

// Concept 3: Thiết kế Database & Indexing (Đã nâng cấp cho Matching)
type Order struct {
	gorm.Model
	CustomerID string `json:"customer_id"`

	// Đánh index cho TrackingCode để tra cứu siêu tốc
	TrackingCode string `gorm:"uniqueIndex" json:"tracking_code"`

	SenderName    string `json:"sender_name"`
	SenderPhone   string `json:"sender_phone"`
	ReceiverName  string `json:"receiver_name"`
	ReceiverPhone string `json:"receiver_phone"`

	// 1. Thông tin hàng hóa (Phục vụ phân loại xe)
	ItemType string  `json:"item_type"` // VD: Thời trang, Hàng dễ vỡ...
	Weight   float64 `json:"weight"`    // Khối lượng để tính tải trọng xe

	// 2. Thông tin Tọa độ/Khu vực (Phục vụ Matching tài xế theo Zone)
	PickupDistrictID   uint `json:"pickup_district_id"`
	PickupWardID       uint `json:"pickup_ward_id"`
	DeliveryDistrictID uint `json:"delivery_district_id"`
	DeliveryWardID     uint `json:"delivery_ward_id"`

	// 3. Chi tiết số nhà, tên đường (Để tài xế nhìn thấy khi nhận đơn)
	PickupAddress   string  `json:"pickup_address"`
	DeliveryAddress string  `json:"delivery_address"`
	TotalFee        float64 `json:"total_fee"`
	// Trạng thái đơn hàng
	Status string `json:"status"` // PENDING, MATCHING, DISPATCHED, COMPLETED
}
