package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username string `gorm:"uniqueIndex" json:"username"`
	Email    string `gorm:"uniqueIndex" json:"email"` // THÊM DÒNG NÀY (Bắt buộc duy nhất)
	Password string `json:"password"`                 // Mật khẩu này sẽ bị mã hóa, không lưu chữ thường
	Role     string `json:"role"`                     // Ví dụ: "customer"
}
