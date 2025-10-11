package concurrency

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"delivery-management-system/internal/services"
)

// Note: These tests focus on concurrency aspects of OrderTracker.
func TestOrderTrackerConcurrency(t *testing.T) {
	// Use a mock database or skip this test if complex setup needed
	t.Skip("Skipping complex concurrency test - requires proper test DB setup")
}

// TestOrderTrackerThreadSafety ensures that OrderTracker methods are thread-safe
func TestOrderTrackerThreadSafety(t *testing.T) {
	cacheService := &services.CacheService{} // Mock cache
	tracker := services.NewOrderTracker(nil, cacheService)

	// Test concurrent AddOrderToTracking
	var wg sync.WaitGroup
	concurrentOperations := 100

	for i := 0; i < concurrentOperations; i++ {
		wg.Add(1)
		go func(orderID uint) {
			defer wg.Done()
			tracker.AddOrderToTracking(orderID)
			// Concurrent read
			tracker.GetActiveOrdersCount()
		}(uint(i))
	}

	wg.Wait()

	// Should not panic and count should be correct
	count := tracker.GetActiveOrdersCount()
	assert.Equal(t, concurrentOperations, count)
	
	t.Logf("✅ Successfully handled %d concurrent operations", concurrentOperations)
}