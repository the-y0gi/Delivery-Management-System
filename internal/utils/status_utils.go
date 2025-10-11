package utils

import "delivery-management-system/internal/models"

// GetStatusDisplayName returns user-friendly status names
func GetStatusDisplayName(status models.OrderStatus) string {
	switch status {
	case models.StatusCreated:
		return "Order Created"
	case models.StatusDispatched:
		return "Dispatched"
	case models.StatusInTransit:
		return "In Transit"
	case models.StatusDelivered:
		return "Delivered"
	case models.StatusCancelled:
		return "Cancelled"
	default:
		return string(status)
	}
}

// GetStatusColor returns color for UI representation
func GetStatusColor(status models.OrderStatus) string {
	switch status {
	case models.StatusCreated:
		return "blue"
	case models.StatusDispatched:
		return "orange"
	case models.StatusInTransit:
		return "purple"
	case models.StatusDelivered:
		return "green"
	case models.StatusCancelled:
		return "red"
	default:
		return "gray"
	}
}