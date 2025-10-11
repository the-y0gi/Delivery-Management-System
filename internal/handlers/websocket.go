package handlers

import (
	"net/http"

	"time"
	"github.com/gin-gonic/gin"
	"delivery-management-system/internal/services"
	"delivery-management-system/internal/utils"
)

type WebSocketHandler struct {
	WebSocketService *services.WebSocketService
	CacheService     *services.CacheService
}

// Handle WebSocket connection
func (wh *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	wh.WebSocketService.HandleWebSocket(c)
}

// Get list of connected WebSocket clients (admin only)
func (wh *WebSocketHandler) GetConnectedClients(c *gin.Context) {
	userRole, _ := c.Get("userRole")
	
	if userRole != "admin" {
		utils.SendError(c, http.StatusForbidden, "Admin access required")
		return
	}

	clientsInfo := wh.WebSocketService.GetClientsInfo()
	utils.SendSuccess(c, http.StatusOK, "Connected WebSocket clients", gin.H{
		"connected_clients": wh.WebSocketService.GetConnectedClientsCount(),
		"clients":          clientsInfo,
	})
}

//  Send system message via WebSocket
func (wh *WebSocketHandler) BroadcastSystemMessage(c *gin.Context) {
	userRole, _ := c.Get("userRole")
	
	if userRole != "admin" {
		utils.SendError(c, http.StatusForbidden, "Admin access required")
		return
	}

	var req struct {
		Message string `json:"message" binding:"required"`
		Type    string `json:"type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	wh.WebSocketService.BroadcastSystemMessage(req.Message, req.Type)

	utils.SendSuccess(c, http.StatusOK, "System message broadcasted", gin.H{
		"message":     req.Message,
		"type":        req.Type,
		"sent_to_clients": wh.WebSocketService.GetConnectedClientsCount(),
	})
}

// Get WebSocket and cache statistics
func (wh *WebSocketHandler) GetRealTimeStats(c *gin.Context) {
	userRole, _ := c.Get("userRole")
	
	if userRole != "admin" {
		utils.SendError(c, http.StatusForbidden, "Admin access required")
		return
	}

	cacheStats, err := wh.CacheService.GetCacheStats()
	if err != nil {
		cacheStats = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	stats := map[string]interface{}{
		"websocket": map[string]interface{}{
			"connected_clients": wh.WebSocketService.GetConnectedClientsCount(),
			"clients_info":     wh.WebSocketService.GetClientsInfo(),
		},
		"cache": cacheStats,
		"timestamp": time.Now(),
	}

	utils.SendSuccess(c, http.StatusOK, "Real-time statistics", stats)
}