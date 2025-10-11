package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"delivery-management-system/internal/models"
)

type CacheService struct {
	client *redis.Client
	ctx    context.Context
}

func NewCacheService(client *redis.Client) *CacheService {
	return &CacheService{
		client: client,
		ctx:    context.Background(),
	}
}

//  CacheOrder with better error handling
func (cs *CacheService) CacheOrder(order *models.Order, expiration time.Duration) error {
	if cs.client == nil {
		log.Printf("⚠️ Redis not available, skipping cache for order %d", order.ID)
		return nil
	}

	key := fmt.Sprintf("order:%d", order.ID)
	
	orderJSON, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to marshal order: %w", err)
	}

	err = cs.client.Set(cs.ctx, key, orderJSON, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to cache order %d: %w", order.ID, err)
	}

	log.Printf("✅ Cached order %d with TTL %v", order.ID, expiration)
	return nil
}

//  GetCachedOrder with better cache miss handling
func (cs *CacheService) GetCachedOrder(orderID uint) (*models.Order, error) {
	if cs.client == nil {
		return nil, nil // Graceful fallback
	}

	key := fmt.Sprintf("order:%d", orderID)
	
	orderJSON, err := cs.client.Get(cs.ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss - not an error
		}
		return nil, fmt.Errorf("failed to get cached order %d: %w", orderID, err)
	}

	var order models.Order
	if err := json.Unmarshal(orderJSON, &order); err != nil {
		// If corrupted data, delete the key
		cs.client.Del(cs.ctx, key)
		return nil, fmt.Errorf("corrupted cache data for order %d: %w", orderID, err)
	}

	log.Printf("✅ Cache hit for order %d", orderID)
	return &order, nil
}

//  InvalidateOrderCache
func (cs *CacheService) InvalidateOrderCache(orderID uint) error {
	if cs.client == nil {
		return nil
	}

	key := fmt.Sprintf("order:%d", orderID)
	err := cs.client.Del(cs.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to invalidate order cache %d: %w", orderID, err)
	}

	log.Printf("✅ Invalidated cache for order %d", orderID)
	return nil
}

//  CacheUserOrders
func (cs *CacheService) CacheUserOrders(userID uint, orders []models.Order, expiration time.Duration) error {
	if cs.client == nil {
		log.Printf("⚠️ Redis not available, skipping cache for user %d orders", userID)
		return nil
	}

	key := fmt.Sprintf("user_orders:%d", userID)
	
	ordersJSON, err := json.Marshal(orders)
	if err != nil {
		return fmt.Errorf("failed to marshal user orders: %w", err)
	}

	err = cs.client.Set(cs.ctx, key, ordersJSON, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to cache user %d orders: %w", userID, err)
	}

	log.Printf("✅ Cached %d orders for user %d with TTL %v", len(orders), userID, expiration)
	return nil
}

//  GetCachedUserOrders
func (cs *CacheService) GetCachedUserOrders(userID uint) ([]models.Order, error) {
	if cs.client == nil {
		return nil, nil
	}

	key := fmt.Sprintf("user_orders:%d", userID)
	
	ordersJSON, err := cs.client.Get(cs.ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get cached user orders %d: %w", userID, err)
	}

	var orders []models.Order
	if err := json.Unmarshal(ordersJSON, &orders); err != nil {
		// If corrupted data, delete the key
		cs.client.Del(cs.ctx, key)
		return nil, fmt.Errorf("corrupted cache data for user %d orders: %w", userID, err)
	}

	log.Printf("✅ Cache hit for user %d orders (%d orders)", userID, len(orders))
	return orders, nil
}

//  InvalidateUserOrdersCache
func (cs *CacheService) InvalidateUserOrdersCache(userID uint) error {
	if cs.client == nil {
		return nil
	}

	key := fmt.Sprintf("user_orders:%d", userID)
	err := cs.client.Del(cs.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to invalidate user %d orders cache: %w", userID, err)
	}

	log.Printf("✅ Invalidated cache for user %d orders", userID)
	return nil
}

//  PublishOrderUpdate with retry mechanism
func (cs *CacheService) PublishOrderUpdate(order *models.Order) error {
	if cs.client == nil {
		log.Printf("⚠️ Redis not available, skipping order update publish for order %d", order.ID)
		return nil
	}

	update := map[string]interface{}{
		"order_id":    order.ID,
		"customer_id": order.CustomerID,
		"status":      order.Status,
		"updated_at":  time.Now(),
		"items":       order.Items,
		"type":        "order_status_update",
	}

	updateJSON, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal order update: %w", err)
	}

	// Retry mechanism for publishing
	var lastErr error
	for i := 0; i < 3; i++ {
		err = cs.client.Publish(cs.ctx, "order_updates", updateJSON).Err()
		if err == nil {
			log.Printf("✅ Published order %d update to Redis (attempt %d)", order.ID, i+1)
			return nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond) // Small delay before retry
	}

	log.Printf("❌ Failed to publish order %d update after 3 attempts: %v", order.ID, lastErr)
	return fmt.Errorf("failed to publish order update after retries: %w", lastErr)
}

//  Get cache statistics
func (cs *CacheService) GetCacheStats() (map[string]interface{}, error) {
	if cs.client == nil {
		return map[string]interface{}{
			"status": "disabled",
		}, nil
	}

	// Get Redis info
	info, err := cs.client.Info(cs.ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}

	// Get keys pattern count (approximate)
	orderKeys, _ := cs.client.Keys(cs.ctx, "order:*").Result()
	userOrderKeys, _ := cs.client.Keys(cs.ctx, "user_orders:*").Result()

	return map[string]interface{}{
		"status":            "connected",
		"total_cached_keys": len(orderKeys) + len(userOrderKeys),
		"cached_orders":     len(orderKeys),
		"cached_user_lists": len(userOrderKeys),
		"redis_info":        info[:200] + "...", // First 200 chars
	}, nil
}

//  Clear all cache (admin function)
func (cs *CacheService) ClearAllCache() error {
	if cs.client == nil {
		return nil
	}

	// Only clear our application keys
	orderKeys, _ := cs.client.Keys(cs.ctx, "order:*").Result()
	userOrderKeys, _ := cs.client.Keys(cs.ctx, "user_orders:*").Result()
	
	allKeys := append(orderKeys, userOrderKeys...)
	
	if len(allKeys) > 0 {
		err := cs.client.Del(cs.ctx, allKeys...).Err()
		if err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
	}

	log.Printf("✅ Cleared all cache (%d keys)", len(allKeys))
	return nil
}

// Enhanced rate limiting with better window management
func (cs *CacheService) CheckRateLimit(key string, limit int, window time.Duration) (bool, int, error) {
	if cs.client == nil {
		return true, 0, nil // No rate limiting if Redis is down
	}

	current, err := cs.client.Get(cs.ctx, key).Int()
	if err != nil && err != redis.Nil {
		return false, 0, err
	}

	if current >= limit {
		return false, current, nil // Rate limit exceeded
	}

	// Use pipeline for atomic operation
	pipe := cs.client.Pipeline()
	incr := pipe.Incr(cs.ctx, key)
	pipe.Expire(cs.ctx, key, window)
	
	_, err = pipe.Exec(cs.ctx)
	if err != nil {
		return false, 0, err
	}

	newCount, _ := incr.Result()
	return true, int(newCount), nil
}

// SubscribeToOrderUpdates returns PubSub connection
func (cs *CacheService) SubscribeToOrderUpdates() *redis.PubSub {
	if cs.client == nil {
		return nil
	}
	return cs.client.Subscribe(cs.ctx, "order_updates")
}