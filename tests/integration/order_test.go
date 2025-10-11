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
	"delivery-management-system/internal/services"
)

// setupOrderTestRouter initializes the Gin router with order routes for testing
func setupOrderTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Initialize handlers
	cacheService := &services.CacheService{} // Mock cache for testing
	orderTracker := services.NewOrderTracker(db, cacheService)
	orderHandler := &handlers.OrderHandler{
		DB:      db,
		Tracker: orderTracker,
		Cache:   cacheService,
	}

	// Protected routes
	protected := router.Group("/api")
	protected.Use(mockAuthMiddleware())
	{
		protected.POST("/orders", orderHandler.CreateOrder)
		protected.GET("/orders", orderHandler.GetUserOrders)
		protected.GET("/orders/:id", orderHandler.GetOrderByID)
		protected.PATCH("/orders/:id/cancel", orderHandler.CancelOrder)
	}

	return router
}

// Mock auth middleware for testing
func mockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set mock user data
		c.Set("userID", uint(1))
		c.Set("userEmail", "test@example.com")
		c.Set("userRole", "customer")
		c.Next()
	}
}

func TestOrderCreation(t *testing.T) {
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

	// Create test user
	user := models.User{
		Email:     "order@test.com",
		Password:  "hashed_password",
		FirstName: "Order",
		LastName:  "User",
		Role:      models.RoleCustomer,
	}
	db.DB.Create(&user)

	router := setupOrderTestRouter(db.DB)

	t.Run("Create Order Success", func(t *testing.T) {
		orderData := map[string]string{
			"items":            "Laptop, Mouse",
			"delivery_address": "123 Test Street",
			"pickup_address":   "456 Warehouse",
		}

		jsonData, _ := json.Marshal(orderData)
		
		req, _ := http.NewRequest("POST", "/api/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Order created successfully", response["message"])
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "created", data["status"])
		assert.Equal(t, "Laptop, Mouse", data["items"])
	})

	t.Run("Get User Orders", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/orders", nil)
		
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)

		assert.True(t, response["success"].(bool))
		
		orders := response["data"].([]interface{})
		assert.Greater(t, len(orders), 0)
	})
}