package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"encoding/json"

	
	"github.com/redis/go-redis/v9"
	"github.com/gin-gonic/gin"
	"delivery-management-system/internal/config"
	"delivery-management-system/internal/database"
	"delivery-management-system/internal/handlers"
	"delivery-management-system/internal/middleware"
	"delivery-management-system/internal/services"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	
	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	
	// Initialize database
	db, err := config.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Initialize Redis
	redisClient, err := config.NewRedisClient(cfg)
	if err != nil {
		log.Printf("⚠️ Failed to connect to Redis: %v", err)
		log.Printf("💡 Continuing without Redis - caching disabled")
		// Continue without Redis - cache service will handle fallback
		redisClient = &config.RedisClient{Client: nil}
	}
	defer redisClient.Close()
	
	// Run migrations
	if err := database.AutoMigrate(db.DB); err != nil {
		log.Fatalf("❌ Failed to run migrations: %v", err)
	}
	
	// Initialize services
	cacheService := services.NewCacheService(redisClient.Client)
	orderTracker := services.NewOrderTracker(db.DB, cacheService)
	webSocketService := services.NewWebSocketService()
	
	// Start background services
	orderTracker.Start()
	// WebSocket service auto-starts its broadcast handler
	
	// Start Redis Pub/Sub listener for real-time updates
	go startRedisPubSubListener(redisClient.Client, webSocketService, cacheService)
	
	// Initialize handlers with all dependencies
	authHandler := &handlers.AuthHandler{DB: db.DB}
	orderHandler := &handlers.OrderHandler{
		DB:      db.DB,
		Tracker: orderTracker,
		Cache:   cacheService,
	}
	webSocketHandler := &handlers.WebSocketHandler{
		WebSocketService: webSocketService,
		CacheService:     cacheService,
	}
	
	// Initialize router
	router := gin.Default()
	
	// Add middleware
	router.Use(loggingMiddleware())
	
	// Public routes
	router.GET("/health", func(c *gin.Context) {
		healthCheck(c, db, redisClient, webSocketService)
	})

	router.GET("/db-check", func(c *gin.Context) {
		var result struct {
			Database string `json:"database"`
			Status   string `json:"status"`
		}
		
		sqlDB, err := db.DB.DB()
		if err != nil {
			c.JSON(500, gin.H{
				"status": "error",
				"message": "Database connection failed",
			})
			return
		}
		
		if err := sqlDB.Ping(); err != nil {
			result.Database = "PostgreSQL"
			result.Status = "disconnected"
			c.JSON(500, result)
			return
		}
		
		result.Database = "PostgreSQL"
		result.Status = "connected"
		c.JSON(200, result)
	})
	
	// WebSocket route (protected - requires authentication)
	router.GET("/ws", middleware.AuthMiddleware(), webSocketHandler.HandleWebSocket)
	
	// Auth routes
	authRoutes := router.Group("/auth")
	{
		authRoutes.POST("/register", authHandler.Register)
		authRoutes.POST("/login", authHandler.Login)
	}
	
	// Protected routes
	protectedRoutes := router.Group("/api")
	protectedRoutes.Use(middleware.AuthMiddleware())
	{
		protectedRoutes.GET("/profile", authHandler.GetProfile)
		protectedRoutes.GET("/orders/status-flow", orderHandler.GetStatusFlow)

		// Order routes
		orderRoutes := protectedRoutes.Group("/orders")
		{
			orderRoutes.POST("/", orderHandler.CreateOrder)
			orderRoutes.GET("/", orderHandler.GetUserOrders)
			orderRoutes.GET("/:id", orderHandler.GetOrderByID)
			orderRoutes.PATCH("/:id/cancel", orderHandler.CancelOrder)
			
			// Admin only routes
			adminRoutes := orderRoutes.Group("")
			adminRoutes.Use(middleware.AdminMiddleware())
			{
				adminRoutes.PATCH("/:id/status", orderHandler.UpdateOrderStatus)
			}
		}

		// Admin dashboard routes
		adminRoutes := protectedRoutes.Group("/admin")
		adminRoutes.Use(middleware.AdminMiddleware())
		{
			adminRoutes.GET("/active-orders", orderHandler.GetActiveOrdersCount)
			adminRoutes.GET("/websocket-clients", webSocketHandler.GetConnectedClients)
			adminRoutes.POST("/broadcast-message", webSocketHandler.BroadcastSystemMessage)
			adminRoutes.GET("/realtime-stats", webSocketHandler.GetRealTimeStats)
			adminRoutes.POST("/clear-cache", func(c *gin.Context) {
				if err := cacheService.ClearAllCache(); err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				c.JSON(200, gin.H{"message": "Cache cleared successfully"})
			})
		}
	}

	// Setup graceful shutdown
	setupGracefulShutdown(orderTracker, redisClient, webSocketService)
	
	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("📊 Order tracker service started")
	log.Printf("🔴 Redis caching service: %s", getRedisStatus(redisClient))
	log.Printf("🔵 WebSocket service started")
	log.Printf("🌐 Environment: %s", cfg.Environment)
	
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}

// Enhanced health check
func healthCheck(c *gin.Context, db *config.Database, redisClient *config.RedisClient, wsService *services.WebSocketService) {
	// Check database
	dbStatus := "connected"
	sqlDB, err := db.DB.DB()
	if err != nil {
		dbStatus = "error"
	} else if err := sqlDB.Ping(); err != nil {
		dbStatus = "disconnected"
	}

	// Check Redis
	redisStatus := "disconnected"
	if redisClient.Client != nil {
		if err := redisClient.Client.Ping(context.Background()).Err(); err == nil {
			redisStatus = "connected"
		}
	} else {
		redisStatus = "not_configured"
	}

	// Get cache stats
	cacheStats := map[string]interface{}{}
	if redisStatus == "connected" {
		cacheService := services.NewCacheService(redisClient.Client)
		stats, err := cacheService.GetCacheStats()
		if err == nil {
			cacheStats = stats
		}
	}

	c.JSON(200, gin.H{
		"status":    "OK",
		"message":   "Delivery Management System is running",
		"timestamp": time.Now(),
		"services": map[string]interface{}{
			"database": map[string]string{
				"status": dbStatus,
				"type":   "PostgreSQL",
			},
			"redis": map[string]interface{}{
				"status": redisStatus,
				"cache":  cacheStats,
			},
			"websocket": map[string]interface{}{
				"connected_clients": wsService.GetConnectedClientsCount(),
				"status":           "running",
			},
			"order_tracker": map[string]interface{}{
				"status": "running",
			},
		},
		"environment": "development", // You can make this config-based
	})
}


// Enhanced Redis Pub/Sub listener - FINAL FIXED VERSION
func startRedisPubSubListener(redisClient *redis.Client, wsService *services.WebSocketService, cacheService *services.CacheService) {
	if redisClient == nil {
		log.Printf("⚠️ Redis not available, skipping Pub/Sub listener")
		return
	}

	log.Printf("🔴 Starting Redis Pub/Sub listener for order updates")
	
	pubsub := redisClient.Subscribe(context.Background(), "order_updates")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for msg := range ch {
		log.Printf("🔴 Received Redis message: %s", msg.Payload)
		
		// Parse the message
		var update map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
			log.Printf("❌ Failed to parse Redis message: %v", err)
			continue
		}
		
		// FIX: Use the public method to broadcast
		wsService.BroadcastRawMessage([]byte(msg.Payload))
		
		// Log the broadcast
		if orderID, exists := update["order_id"]; exists {
			log.Printf("🔵 Broadcasted order %v update to WebSocket clients", orderID)
		}
	}
}

// Logging middleware
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Process request
		c.Next()
		
		// Log after request completion
		duration := time.Since(start)
		log.Printf("%s %s %d %v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	}
}

// Helper function to get Redis status
func getRedisStatus(redisClient *config.RedisClient) string {
	if redisClient.Client == nil {
		return "not_configured"
	}
	if err := redisClient.Client.Ping(context.Background()).Err(); err != nil {
		return "disconnected"
	}
	return "connected"
}

// Enhanced graceful shutdown
func setupGracefulShutdown(tracker *services.OrderTracker, redis *config.RedisClient, ws *services.WebSocketService) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		log.Println("🛑 Shutting down gracefully...")
		
		// Stop services in order
		log.Println("⏳ Stopping order tracker...")
		tracker.Stop()
		
		log.Println("⏳ Stopping WebSocket service...")
		ws.Shutdown()
		
		log.Println("⏳ Closing Redis connection...")
		redis.Close()
		
		log.Println("✅ All services stopped successfully")
		os.Exit(0)
	}()
}