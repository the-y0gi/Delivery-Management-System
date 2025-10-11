package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"delivery-management-system/internal/config"
	"delivery-management-system/internal/database"
	"delivery-management-system/internal/handlers"
	"delivery-management-system/internal/models"
)
// setupTestRouter initializes the Gin router with auth routes for testing	
func setupTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	authHandler := &handlers.AuthHandler{DB: db}
	
	router.POST("/auth/register", authHandler.Register)
	router.POST("/auth/login", authHandler.Login)

	return router
}

// TestAuthLogin tests the login functionality of the AuthHandler
func TestAuthRegistration(t *testing.T) {
	// Setup test database
	cfg := &config.Config{
		DatabaseURL: "postgres://user:password@localhost:5432/delivery_test?sslmode=disable",
	}
	
	db, err := config.NewDatabase(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	// Clean database
	db.DB.Migrator().DropTable(&models.User{}, &models.Order{})
	database.AutoMigrate(db.DB)

	router := setupTestRouter(db.DB)

	t.Run("Successful Registration", func(t *testing.T) {
		registerData := map[string]string{
			"email":      "test@example.com",
			"password":   "password123",
			"first_name": "John",
			"last_name":  "Doe",
		}

		jsonData, _ := json.Marshal(registerData)
		
		req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "User registered successfully", response["message"])
		
		data := response["data"].(map[string]interface{})
		assert.NotEmpty(t, data["token"])
		assert.Equal(t, "test@example.com", data["user"].(map[string]interface{})["email"])
	})

	t.Run("Duplicate Email Registration", func(t *testing.T) {
		registerData := map[string]string{
			"email":      "test@example.com", // Same email
			"password":   "password123",
			"first_name": "Jane",
			"last_name":  "Doe",
		}

		jsonData, _ := json.Marshal(registerData)
		
		req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"], "already exists")
	})
}

// TestAuthLogin tests the login functionality
func TestAuthLogin(t *testing.T) {
	// Setup test database
	cfg := &config.Config{
		DatabaseURL: "postgres://user:password@localhost:5432/delivery_test?sslmode=disable",
	}
	
	db, err := config.NewDatabase(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	db.DB.Migrator().DropTable(&models.User{}, &models.Order{})
	database.AutoMigrate(db.DB)

	// First register a user using the handler (instead of direct DB creation)
	router := setupTestRouter(db.DB)
	
	// Register a user first
	registerData := map[string]string{
		"email":      "login@test.com",
		"password":   "password123",
		"first_name": "Login",
		"last_name":  "User",
	}
	jsonData, _ := json.Marshal(registerData)
	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	// End of registration

	t.Run("Successful Login", func(t *testing.T) {
		loginData := map[string]string{
			"email":    "login@test.com",
			"password": "password123",
		}

		jsonData, _ := json.Marshal(loginData)
		
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Login successful", response["message"])
		assert.NotEmpty(t, response["data"].(map[string]interface{})["token"])
	})

	t.Run("Invalid Login", func(t *testing.T) {
		loginData := map[string]string{
			"email":    "login@test.com",
			"password": "wrongpassword",
		}

		jsonData, _ := json.Marshal(loginData)
		
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"], "Invalid email or password")
	})
}