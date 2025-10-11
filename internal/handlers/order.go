package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"delivery-management-system/internal/models"
	"delivery-management-system/internal/services"
	"delivery-management-system/internal/utils"
)

type OrderHandler struct {
	DB *gorm.DB
		Tracker *services.OrderTracker
		Cache   *services.CacheService
}

type CreateOrderRequest struct {
	Items           string `json:"items" binding:"required"`
	DeliveryAddress string `json:"delivery_address" binding:"required"`
	PickupAddress   string `json:"pickup_address,omitempty"`
}

type UpdateOrderRequest struct {
	Status models.OrderStatus `json:"status" binding:"required,oneof=created dispatched in_transit delivered cancelled"`
}

// GetUserOrders with Redis caching
func (h *OrderHandler) GetUserOrders(c *gin.Context) {
	userID, _ := c.Get("userID")
	userRole, _ := c.Get("userRole")

	// Try to get from cache first (for customers only)
	if userRole == "customer" {
		cachedOrders, err := h.Cache.GetCachedUserOrders(userID.(uint))
		if err != nil {
			log.Printf("❌ Cache error for user %d orders: %v", userID.(uint), err)
		}
		if cachedOrders != nil {
			log.Printf("✅ Serving user %d orders from cache", userID.(uint))
			utils.SendSuccess(c, http.StatusOK, "Orders retrieved from cache", cachedOrders)
			return
		}
	}

	// Cache miss - fetch from database
	var orders []models.Order
	query := h.DB.Preload("Customer")
	
	if userRole == "customer" {
		query = query.Where("customer_id = ?", userID)
	}

	if err := query.Order("created_at DESC").Find(&orders).Error; err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to fetch orders")
		return
	}

	// Cache the results (for customers only)
	if userRole == "customer" {
		go func(userID uint, orders []models.Order) {
			if err := h.Cache.CacheUserOrders(userID, orders, 2*time.Minute); err != nil {
				log.Printf("❌ Failed to cache user %d orders: %v", userID, err)
			} else {
				log.Printf("✅ Cached user %d orders (%d orders)", userID, len(orders))
			}
		}(userID.(uint), orders)
	}

	log.Printf("✅ Serving user %d orders from database", userID.(uint))
	utils.SendSuccess(c, http.StatusOK, "Orders retrieved successfully", orders)
}

// Update GetOrderByID with caching
func (h *OrderHandler) GetOrderByID(c *gin.Context) {
	userID, _ := c.Get("userID")
	userRole, _ := c.Get("userRole")

	orderIDStr := c.Param("id")

	// Convert orderID string to uint
	orderIDUint, err := strconv.ParseUint(orderIDStr, 10, 64)
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid order ID")
		return
	}

	// Try cache first
	cachedOrder, err := h.Cache.GetCachedOrder(uint(orderIDUint))
	if err != nil {
		log.Printf("Cache error: %v", err)
	}
	if cachedOrder != nil {
		// Check authorization for cached order
		if userRole == "customer" && cachedOrder.CustomerID != userID {
			utils.SendError(c, http.StatusForbidden, "Access denied")
			return
		}
		utils.SendSuccess(c, http.StatusOK, "Order retrieved from cache", cachedOrder)
		return
	}

	var order models.Order
	if err := h.DB.Preload("Customer").First(&order, uint(orderIDUint)).Error; err != nil {
		utils.SendError(c, http.StatusNotFound, "Order not found")
		return
	}

	// Check if user owns the order or is admin
	if userRole == "customer" && order.CustomerID != userID {
		utils.SendError(c, http.StatusForbidden, "Access denied")
		return
	}

	// Cache the order
	go func() {
		if err := h.Cache.CacheOrder(&order, 5*time.Minute); err != nil {
			log.Printf("Failed to cache order: %v", err)
		}
	}()

	utils.SendSuccess(c, http.StatusOK, "Order retrieved successfully", order)
}

// CreateOrder with cache invalidation
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID, _ := c.Get("userID")
	// userRole, _ := c.Get("userRole")

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Create order
	order := models.Order{
		CustomerID:      userID.(uint),
		Items:           req.Items,
		DeliveryAddress: req.DeliveryAddress,
		PickupAddress:   req.PickupAddress,
		Status:          models.StatusCreated,
	}

	if err := h.DB.Create(&order).Error; err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to create order")
		return
	}

	// Add to background tracking
	h.Tracker.AddOrderToTracking(order.ID)
	
	// Invalidate user orders cache
	go func(userID uint) {
		if err := h.Cache.InvalidateUserOrdersCache(userID); err != nil {
			log.Printf("❌ Failed to invalidate user %d orders cache: %v", userID, err)
		} else {
			log.Printf("✅ Invalidated user %d orders cache", userID)
		}
	}(userID.(uint))

	// Load customer details for response
	h.DB.Preload("Customer").First(&order)

	// Publish real-time update
	go func(order models.Order) {
		if err := h.Cache.PublishOrderUpdate(&order); err != nil {
			log.Printf("❌ Failed to publish order %d update: %v", order.ID, err)
		} else {
			log.Printf("✅ Published order %d creation update", order.ID)
		}
	}(order)

	log.Printf("✅ Created order %d for user %d", order.ID, userID.(uint))
	utils.SendSuccess(c, http.StatusCreated, "Order created successfully", order)
}

// CancelOrder with cache invalidation
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	userID, _ := c.Get("userID")
	userRole, _ := c.Get("userRole")

	orderID := c.Param("id")
	
	var order models.Order
	if err := h.DB.First(&order, orderID).Error; err != nil {
		utils.SendError(c, http.StatusNotFound, "Order not found")
		return
	}

	// Check if user owns the order or is admin
	if userRole == "customer" && order.CustomerID != userID {
		utils.SendError(c, http.StatusForbidden, "Access denied")
		return
	}

	// Check if order is already in final state
	if order.Status.IsFinalStatus() {
		utils.SendError(c, http.StatusBadRequest, 
			fmt.Sprintf("Cannot cancel order with status: %s", order.Status))
		return
	}

	// Validate cancellation transition
	if err := order.Status.ValidateTransition(models.StatusCancelled); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Update order status
	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.StatusCancelled,
		"cancelled_at": &now,
	}

	if err := h.DB.Model(&order).Updates(updates).Error; err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to cancel order")
		return
	}

	// Remove from background tracking
	h.Tracker.RemoveOrderFromTracking(order.ID)

	// Invalidate caches
	go func(order models.Order, userID interface{}) {
		// Invalidate order cache
		if err := h.Cache.InvalidateOrderCache(order.ID); err != nil {
			log.Printf("❌ Failed to invalidate order %d cache: %v", order.ID, err)
		} else {
			log.Printf("✅ Invalidated order %d cache", order.ID)
		}
		
		// Invalidate user orders cache
		if err := h.Cache.InvalidateUserOrdersCache(order.CustomerID); err != nil {
			log.Printf("❌ Failed to invalidate user %d orders cache: %v", order.CustomerID, err)
		} else {
			log.Printf("✅ Invalidated user %d orders cache", order.CustomerID)
		}
	}(order, userID)

	// Publish real-time update
	go func(order models.Order) {
		if err := h.Cache.PublishOrderUpdate(&order); err != nil {
			log.Printf("❌ Failed to publish order %d cancellation: %v", order.ID, err)
		} else {
			log.Printf("✅ Published order %d cancellation update", order.ID)
		}
	}(order)

	log.Printf("✅ Cancelled order %d by user %d", order.ID, userID.(uint))
	utils.SendSuccess(c, http.StatusOK, "Order cancelled successfully", order)
}

// UpdateOrderStatus with cache updates and real-time notifications
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	userRole, _ := c.Get("userRole")

	// Only admin can update order status
	if userRole != "admin" {
		utils.SendError(c, http.StatusForbidden, "Admin access required")
		return
	}

	orderID := c.Param("id")
	
	var req UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	var order models.Order
	if err := h.DB.First(&order, orderID).Error; err != nil {
		utils.SendError(c, http.StatusNotFound, "Order not found")
		return
	}

	// Validate status transition
	if err := order.Status.ValidateTransition(req.Status); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Update order status with timestamp
	updates := map[string]interface{}{
		"status": req.Status,
	}

	now := time.Now()
	switch req.Status {
	case models.StatusDispatched:
		updates["dispatched_at"] = &now
	case models.StatusInTransit:
		updates["in_transit_at"] = &now
	case models.StatusDelivered:
		updates["delivered_at"] = &now
		// Remove from tracking when delivered
		defer h.Tracker.RemoveOrderFromTracking(order.ID)
	case models.StatusCancelled:
		updates["cancelled_at"] = &now
		// Remove from tracking when cancelled
		defer h.Tracker.RemoveOrderFromTracking(order.ID)
	}

	if err := h.DB.Model(&order).Updates(updates).Error; err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to update order status")
		return
	}

	// Refresh order data
	h.DB.Preload("Customer").First(&order, orderID)

	// Invalidate caches
	go func(order models.Order) {
		// Invalidate order cache
		if err := h.Cache.InvalidateOrderCache(order.ID); err != nil {
			log.Printf("❌ Failed to invalidate order %d cache: %v", order.ID, err)
		} else {
			log.Printf("✅ Invalidated order %d cache", order.ID)
		}
		
		// Invalidate user orders cache
		if err := h.Cache.InvalidateUserOrdersCache(order.CustomerID); err != nil {
			log.Printf("❌ Failed to invalidate user %d orders cache: %v", order.CustomerID, err)
		} else {
			log.Printf("✅ Invalidated user %d orders cache", order.CustomerID)
		}
	}(order)

	// Publish real-time update via Redis
	go func(order models.Order) {
		if err := h.Cache.PublishOrderUpdate(&order); err != nil {
			log.Printf("❌ Failed to publish order %d status update: %v", order.ID, err)
		} else {
			log.Printf("✅ Published order %d status update to %s", order.ID, order.Status)
		}
	}(order)

	log.Printf("✅ Updated order %d status to %s by admin", order.ID, req.Status)
	utils.SendSuccess(c, http.StatusOK, "Order status updated successfully", order)
}

// GetStatusFlow returns the order status flow
func (h *OrderHandler) GetStatusFlow(c *gin.Context) {
	statusFlow := models.GetStatusFlow()
	utils.SendSuccess(c, http.StatusOK, "Status flow retrieved", statusFlow)
}

// GetActiveOrdersCount returns count of actively tracked orders
func (h *OrderHandler) GetActiveOrdersCount(c *gin.Context) {
	userRole, _ := c.Get("userRole")
	
	if userRole != "admin" {
		utils.SendError(c, http.StatusForbidden, "Admin access required")
		return
	}

	count := h.Tracker.GetActiveOrdersCount()
	utils.SendSuccess(c, http.StatusOK, "Active orders count", gin.H{
		"active_orders": count,
	})
}