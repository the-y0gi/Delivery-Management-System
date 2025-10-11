package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"delivery-management-system/internal/auth"
)

// TestPasswordHashing tests the password hashing and verification functions
func TestPasswordHashing(t *testing.T) {
	password := "testPassword123"
	
	hash, err := auth.HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// Test correct password
	isValid := auth.CheckPasswordHash(password, hash)
	assert.True(t, isValid)

	// Test wrong password
	isValid = auth.CheckPasswordHash("wrongPassword", hash)
	assert.False(t, isValid)
}
// TestJWTGeneration tests the generation and validation of JWTs
func TestJWTGeneration(t *testing.T) {
	userID := uint(1)
	email := "test@example.com"
	role := "customer"

	token, err := auth.GenerateJWT(userID, email, role)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	claims, err := auth.ValidateJWT(token)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, role, claims.Role)
}

// TestInvalidJWT tests the validation of an invalid JWT
func TestInvalidJWT(t *testing.T) {
	// Test invalid token
	_, err := auth.ValidateJWT("invalid.token.here")
	assert.Error(t, err)

	// Test empty token
	_, err = auth.ValidateJWT("")
	assert.Error(t, err)
}