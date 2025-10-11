package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"delivery-management-system/internal/models"
)

// TestOrderStatusTransitions tests valid and invalid status transitions
func TestOrderStatusTransitions(t *testing.T) {
	tests := []struct {
		name        string
		current     models.OrderStatus
		next        models.OrderStatus
		shouldError bool
	}{
		{
			name:        "Valid: Created to Dispatched",
			current:     models.StatusCreated,
			next:        models.StatusDispatched,
			shouldError: false,
		},
		{
			name:        "Valid: Dispatched to InTransit",
			current:     models.StatusDispatched,
			next:        models.StatusInTransit,
			shouldError: false,
		},
		{
			name:        "Valid: InTransit to Delivered",
			current:     models.StatusInTransit,
			next:        models.StatusDelivered,
			shouldError: false,
		},
		{
			name:        "Valid: Created to Cancelled",
			current:     models.StatusCreated,
			next:        models.StatusCancelled,
			shouldError: false,
		},
		{
			name:        "Invalid: Created to Delivered",
			current:     models.StatusCreated,
			next:        models.StatusDelivered,
			shouldError: true,
		},
		{
			name:        "Invalid: Delivered to Cancelled",
			current:     models.StatusDelivered,
			next:        models.StatusCancelled,
			shouldError: true,
		},
		{
			name:        "Invalid: Cancelled to Dispatched",
			current:     models.StatusCancelled,
			next:        models.StatusDispatched,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.current.ValidateTransition(tt.next)
			
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFinalStatusCheck tests the final status check for orders
func TestFinalStatusCheck(t *testing.T) {
	tests := []struct {
		status  models.OrderStatus
		isFinal bool
	}{
		{models.StatusCreated, false},
		{models.StatusDispatched, false},
		{models.StatusInTransit, false},
		{models.StatusDelivered, true},
		{models.StatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.isFinal, tt.status.IsFinalStatus())
		})
	}
}

// TestGetNextStatus tests the retrieval of the next logical status
func TestGetNextStatus(t *testing.T) {
	tests := []struct {
		name        string
		current     models.OrderStatus
		expected    models.OrderStatus
		shouldError bool
	}{
		{
			name:        "Created should go to Dispatched",
			current:     models.StatusCreated,
			expected:    models.StatusDispatched,
			shouldError: false,
		},
		{
			name:        "Dispatched should go to InTransit",
			current:     models.StatusDispatched,
			expected:    models.StatusInTransit,
			shouldError: false,
		},
		{
			name:        "InTransit should go to Delivered",
			current:     models.StatusInTransit,
			expected:    models.StatusDelivered,
			shouldError: false,
		},
		{
			name:        "Delivered has no next status",
			current:     models.StatusDelivered,
			expected:    "",
			shouldError: true,
		},
		{
			name:        "Cancelled has no next status",
			current:     models.StatusCancelled,
			expected:    "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := tt.current.GetNextStatus()
			
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, next)
			}
		})
	}
}