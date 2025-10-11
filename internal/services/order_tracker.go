package services

import (
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
	"delivery-management-system/internal/models"
)

type OrderTracker struct {
	DB          *gorm.DB
	activeOrders sync.Map
	stopChan    chan struct{}
	wg          sync.WaitGroup
	Cache       *CacheService
}

// NewOrderTracker initializes the order tracker service
func NewOrderTracker(db *gorm.DB, cache *CacheService) *OrderTracker {
	return &OrderTracker{
		DB:       db,
		stopChan: make(chan struct{}),
		Cache:    cache,
	}
}

// Start the order tracker service
func (ot *OrderTracker) Start() {
	log.Println("Starting order tracker service...")
	
	ot.wg.Add(1)
	go ot.trackOrders()
}

// Stop the order tracker service gracefully
func (ot *OrderTracker) Stop() {
	log.Println("Stopping order tracker service...")
	close(ot.stopChan)
	ot.wg.Wait()
	log.Println("Order tracker service stopped")
}

// AddOrderToTracking adds an order to active tracking
func (ot *OrderTracker) AddOrderToTracking(orderID uint) {
	ot.activeOrders.Store(orderID, true)
	log.Printf("Order %d added to tracking", orderID)
}

// RemoveOrderFromTracking removes an order from active tracking
func (ot *OrderTracker) RemoveOrderFromTracking(orderID uint) {
	ot.activeOrders.Delete(orderID)
	log.Printf("Order %d removed from tracking", orderID)
}

// trackOrders periodically processes active orders
func (ot *OrderTracker) trackOrders() {
	defer ot.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ot.processOrders()
		case <-ot.stopChan:
			return
		}
	}
}

// processOrders processes all active orders concurrently
func (ot *OrderTracker) processOrders() {
	var orderIDs []uint
	
	ot.activeOrders.Range(func(key, value interface{}) bool {
		orderID := key.(uint)
		orderIDs = append(orderIDs, orderID)
		return true
	})

	if len(orderIDs) == 0 {
		return
	}

	log.Printf("Processing %d active orders", len(orderIDs))

	var wg sync.WaitGroup
	for _, orderID := range orderIDs {
		wg.Add(1)
		go ot.processSingleOrder(orderID, &wg)
	}
	wg.Wait()
}

// processSingleOrder handles status updates for a single order
func (ot *OrderTracker) processSingleOrder(orderID uint, wg *sync.WaitGroup) {
	defer wg.Done()

	var order models.Order
	if err := ot.DB.First(&order, orderID).Error; err != nil {
		log.Printf("Error fetching order %d: %v", orderID, err)
		ot.RemoveOrderFromTracking(orderID)
		return
	}

	if order.Status.IsFinalStatus() {
		ot.RemoveOrderFromTracking(orderID)
		return
	}

	switch order.Status {
	case models.StatusCreated:
		ot.updateOrderStatus(orderID, models.StatusDispatched)
		
	case models.StatusDispatched:
		ot.updateOrderStatus(orderID, models.StatusInTransit)
		
	case models.StatusInTransit:
		ot.updateOrderStatus(orderID, models.StatusDelivered)
	}
}


// updateOrderStatus with Redis publishing
func (ot *OrderTracker) updateOrderStatus(orderID uint, newStatus models.OrderStatus) {
	var order models.Order
	
	err := ot.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}

		if err := order.Status.ValidateTransition(newStatus); err != nil {
			return err
		}

		updates := map[string]interface{}{
			"status": newStatus,
		}

		now := time.Now()
		switch newStatus {
		case models.StatusDispatched:
			updates["dispatched_at"] = &now
		case models.StatusInTransit:
			updates["in_transit_at"] = &now
		case models.StatusDelivered:
			updates["delivered_at"] = &now
			defer ot.RemoveOrderFromTracking(orderID)
		}

		return tx.Model(&order).Updates(updates).Error
	})

	if err != nil {
		log.Printf("❌ Error updating order %d status to %s: %v", orderID, newStatus, err)
	} else {
		log.Printf("✅ Order %d status updated to %s", orderID, newStatus)
		
		// FIX: Publish to Redis for real-time updates
		if ot.Cache != nil {
			// Reload the order with customer data for complete update
			var updatedOrder models.Order
			if err := ot.DB.Preload("Customer").First(&updatedOrder, orderID).Error; err == nil {
				go func() {
					if err := ot.Cache.PublishOrderUpdate(&updatedOrder); err != nil {
						log.Printf("❌ Failed to publish order %d auto-update: %v", orderID, err)
					} else {
						log.Printf("✅ Published automatic order %d status update to %s", orderID, newStatus)
					}
					
					// Also invalidate cache
					ot.Cache.InvalidateOrderCache(orderID)
					ot.Cache.InvalidateUserOrdersCache(updatedOrder.CustomerID)
				}()
			}
		}
	}
}

// GetActiveOrdersCount returns the count of currently tracked active orders
func (ot *OrderTracker) GetActiveOrdersCount() int {
	count := 0
	ot.activeOrders.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}