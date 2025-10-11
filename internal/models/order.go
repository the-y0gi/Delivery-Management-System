package models

import (
	"time"
	
	"gorm.io/gorm"
)

type OrderStatus string

const (
	StatusCreated    OrderStatus = "created"
	StatusDispatched OrderStatus = "dispatched"
	StatusInTransit  OrderStatus = "in_transit"
	StatusDelivered  OrderStatus = "delivered"
	StatusCancelled  OrderStatus = "cancelled"
)

type Order struct {
	ID          uint        `json:"id" gorm:"primaryKey"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
	
	CustomerID  uint        `json:"customer_id" gorm:"not null"`
	Customer    User        `json:"customer" gorm:"foreignKey:CustomerID"`
	
	Items       string      `json:"items" gorm:"type:text;not null"` // JSON string of items
	Status      OrderStatus `json:"status" gorm:"type:varchar(20);default:created"`
	
	// Delivery details
	PickupAddress  string    `json:"pickup_address" gorm:"type:text"`
	DeliveryAddress string   `json:"delivery_address" gorm:"type:text;not null"`
	
	// Timestamps for status changes
	DispatchedAt  *time.Time `json:"dispatched_at,omitempty"`
	InTransitAt   *time.Time `json:"in_transit_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	CancelledAt   *time.Time `json:"cancelled_at,omitempty"`
}

// TableName specifies the table name for Order
func (Order) TableName() string {
	return "orders"
}