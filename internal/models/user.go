package models

import (
	"time"
	
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleCustomer UserRole = "customer"
	RoleAdmin    UserRole = "admin"
)

type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
	
	Email     string   `json:"email" gorm:"uniqueIndex;not null"`
	Password  string   `json:"-" gorm:"not null"` // Password hash, never expose in JSON
	FirstName string   `json:"first_name" gorm:"not null"`
	LastName  string   `json:"last_name" gorm:"not null"`
	Role      UserRole `json:"role" gorm:"type:varchar(20);default:customer"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}