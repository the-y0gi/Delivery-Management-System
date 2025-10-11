package models

import (
	"errors"
	"fmt"
)

// Valid status transitions
var validTransitions = map[OrderStatus][]OrderStatus{
	StatusCreated:    {StatusDispatched, StatusCancelled},
	StatusDispatched: {StatusInTransit, StatusCancelled},
	StatusInTransit:  {StatusDelivered, StatusCancelled},
	StatusDelivered:  {},
	StatusCancelled:  {},
}

// CanTransitionTo checks if a status transition is valid
func (current OrderStatus) CanTransitionTo(new OrderStatus) bool {
	allowedTransitions, exists := validTransitions[current]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == new {
			return true
		}
	}
	return false
}

// ValidateTransition validates and returns error if transition is invalid
func (current OrderStatus) ValidateTransition(new OrderStatus) error {
	if !current.CanTransitionTo(new) {
		return fmt.Errorf("invalid status transition: %s → %s", current, new)
	}
	return nil
}

// GetNextStatus returns the next logical status in the delivery flow
func (current OrderStatus) GetNextStatus() (OrderStatus, error) {
	transitions, exists := validTransitions[current]
	if !exists || len(transitions) == 0 {
		return "", errors.New("no valid next status")
	}

	// Return the first non-cancellation status if available
	for _, status := range transitions {
		if status != StatusCancelled {
			return status, nil
		}
	}
	
	// If only cancellation is available, return it
	if len(transitions) > 0 {
		return transitions[0], nil
	}
	
	return "", errors.New("no valid next status")
}

// IsFinalStatus checks if the status is a final state
func (s OrderStatus) IsFinalStatus() bool {
	return s == StatusDelivered || s == StatusCancelled
}

// GetStatusFlow returns the complete status flow
func GetStatusFlow() []OrderStatus {
	return []OrderStatus{
		StatusCreated,
		StatusDispatched,
		StatusInTransit,
		StatusDelivered,
	}
}