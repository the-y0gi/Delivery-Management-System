package services

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"delivery-management-system/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Client struct {
	Conn     *websocket.Conn
	UserID   uint
	UserRole string
}

type WebSocketService struct {
	clients    map[*websocket.Conn]*Client
	clientsMux sync.RWMutex
	broadcast  chan []byte
}

// NewWebSocketService initializes the WebSocket service
func NewWebSocketService() *WebSocketService {
	service := &WebSocketService{
		clients:   make(map[*websocket.Conn]*Client),
		broadcast: make(chan []byte),
	}
	
	go service.handleBroadcasts()
	log.Println("🔵 WebSocket service started with broadcast handler")
	return service
}

// Add this method to WebSocketService for raw message broadcasting
func (ws *WebSocketService) BroadcastRawMessage(message []byte) {
    ws.broadcast <- message
}

// Enhanced HandleWebSocket with authentication
func (ws *WebSocketService) HandleWebSocket(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userID, exists := c.Get("userID")
	userRole, roleExists := c.Get("userRole")
	
	if !exists || !roleExists {
		log.Println("❌ WebSocket connection rejected: unauthenticated")
		c.JSON(401, gin.H{"error": "Authentication required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade error: %v", err)
		return
	}

	// Create client with user info
	client := &Client{
		Conn:     conn,
		UserID:   userID.(uint),
		UserRole: userRole.(string),
	}

	// Register client
	ws.clientsMux.Lock()
	ws.clients[conn] = client
	ws.clientsMux.Unlock()

	log.Printf("🔵 WebSocket client connected. UserID: %d, Role: %s, Total clients: %d", 
		client.UserID, client.UserRole, len(ws.clients))

	// Send welcome message
	ws.sendWelcomeMessage(client)

	// Handle messages from client
	go ws.handleClientMessages(client)
}

// handleClientMessages processes incoming messages from a client
func (ws *WebSocketService) handleClientMessages(client *Client) {
	defer ws.removeClient(client.Conn)

	for {
		messageType, p, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}

		// Handle different message types
		switch messageType {
		case websocket.TextMessage:
			ws.handleTextMessage(client, p)
		case websocket.PingMessage:
			client.Conn.WriteMessage(websocket.PongMessage, nil)
		}
	}
}

// handleTextMessage processes text messages from clients
func (ws *WebSocketService) handleTextMessage(client *Client, message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("❌ Invalid WebSocket message from user %d: %v", client.UserID, err)
		return
	}

	// Handle different message types from client
	switch msg["type"] {
	case "ping":
		ws.sendPong(client)
	case "subscribe_orders":
		ws.handleOrderSubscription(client, msg)
	case "unsubscribe_orders":
		ws.handleOrderUnsubscription(client, msg)
	default:
		log.Printf("⚠️ Unknown WebSocket message type from user %d: %s", client.UserID, msg["type"])
	}
}

// sendWelcomeMessage sends a welcome message to the connected client
func (ws *WebSocketService) sendWelcomeMessage(client *Client) {
	welcomeMsg := map[string]interface{}{
		"type":    "welcome",
		"user_id": client.UserID,
		"role":    client.UserRole,
		"message": "Connected to Delivery Management System WebSocket",
		"timestamp": time.Now(),
	}

	msgBytes, _ := json.Marshal(welcomeMsg)
	client.Conn.WriteMessage(websocket.TextMessage, msgBytes)
}

// sendPong responds to ping messages
func (ws *WebSocketService) sendPong(client *Client) {
	pongMsg := map[string]interface{}{
		"type": "pong",
		"timestamp": time.Now(),
	}

	msgBytes, _ := json.Marshal(pongMsg)
	client.Conn.WriteMessage(websocket.TextMessage, msgBytes)
}

// handleOrderSubscription handles subscription requests
func (ws *WebSocketService) handleOrderSubscription(client *Client, msg map[string]interface{}) {
	// In future, can implement specific order subscriptions
	subscriptionMsg := map[string]interface{}{
		"type": "subscription_confirmed",
		"subscription": "order_updates",
		"message": "Subscribed to order updates",
		"timestamp": time.Now(),
	}

	msgBytes, _ := json.Marshal(subscriptionMsg)
	client.Conn.WriteMessage(websocket.TextMessage, msgBytes)
	
	log.Printf("🔵 User %d subscribed to order updates", client.UserID)
}

// handleOrderUnsubscription handles unsubscription requests
func (ws *WebSocketService) handleOrderUnsubscription(client *Client, msg map[string]interface{}) {
	unsubscriptionMsg := map[string]interface{}{
		"type": "unsubscription_confirmed",
		"message": "Unsubscribed from order updates",
		"timestamp": time.Now(),
	}

	msgBytes, _ := json.Marshal(unsubscriptionMsg)
	client.Conn.WriteMessage(websocket.TextMessage, msgBytes)
	
	log.Printf("🔵 User %d unsubscribed from order updates", client.UserID)
}

// removeClient cleans up when a client disconnects
func (ws *WebSocketService) removeClient(conn *websocket.Conn) {
	ws.clientsMux.Lock()
	client := ws.clients[conn]
	delete(ws.clients, conn)
	ws.clientsMux.Unlock()

	if client != nil {
		log.Printf("🔵 WebSocket client disconnected. UserID: %d, Total clients: %d", 
			client.UserID, len(ws.clients))
	}

	conn.Close()
}

//  BroadcastOrderUpdate with user filtering
func (ws *WebSocketService) BroadcastOrderUpdate(order *models.Order) {
	update := map[string]interface{}{
		"type":       "order_update",
		"order_id":   order.ID,
		"status":     order.Status,
		"customer_id": order.CustomerID,
		"items":      order.Items,
		"timestamp":  time.Now(),
	}

	message, err := json.Marshal(update)
	if err != nil {
		log.Printf("❌ Error marshaling order update: %v", err)
		return
	}

	ws.clientsMux.RLock()
	defer ws.clientsMux.RUnlock()

	sentCount := 0
	for conn, client := range ws.clients {
		// Send to:
		// - Order owner (customer)
		// - All admin users
		// - Users with appropriate permissions
		if client.UserID == order.CustomerID || client.UserRole == "admin" {
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				conn.Close()
				delete(ws.clients, conn)
			} else {
				sentCount++
			}
		}
	}

	log.Printf("🔵 Broadcast order %d update to %d/%d clients", 
		order.ID, sentCount, len(ws.clients))
}

// BroadcastSystemMessage for admin announcements
func (ws *WebSocketService) BroadcastSystemMessage(message string, messageType string) {
	systemMsg := map[string]interface{}{
		"type":      "system_message",
		"message":   message,
		"sub_type":  messageType,
		"timestamp": time.Now(),
	}

	msgBytes, _ := json.Marshal(systemMsg)

	ws.clientsMux.RLock()
	defer ws.clientsMux.RUnlock()

	sentCount := 0
	for conn := range ws.clients {
		// Send to all connected clients
		err := conn.WriteMessage(websocket.TextMessage, msgBytes)
		if err != nil {
			conn.Close()
			delete(ws.clients, conn)
		} else {
			sentCount++
		}
	}

	log.Printf("🔵 Broadcast system message to %d clients: %s", sentCount, message)
}

// handleBroadcasts listens for broadcast messages and sends them to all clients
func (ws *WebSocketService) handleBroadcasts() {
	for {
		message := <-ws.broadcast
		
		ws.clientsMux.RLock()
		for client := range ws.clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				client.Close()
				delete(ws.clients, client)
			}
		}
		ws.clientsMux.RUnlock()
	}
}

// GetConnectedClientsCount returns the number of connected clients
func (ws *WebSocketService) GetConnectedClientsCount() int {
	ws.clientsMux.RLock()
	defer ws.clientsMux.RUnlock()
	return len(ws.clients)
}

// GetClientsInfo returns info about connected clients
func (ws *WebSocketService) GetClientsInfo() []map[string]interface{} {
	ws.clientsMux.RLock()
	defer ws.clientsMux.RUnlock()

	clientsInfo := make([]map[string]interface{}, 0)
	for _, client := range ws.clients {
		clientsInfo = append(clientsInfo, map[string]interface{}{
			"user_id": client.UserID,
			"user_role": client.UserRole,
			"connected_at": "recent", // Could track actual connection time
		})
	}
	return clientsInfo
}

// Graceful shutdown
func (ws *WebSocketService) Shutdown() {
	log.Println("🔵 Shutting down WebSocket service...")
	
	ws.clientsMux.Lock()
	defer ws.clientsMux.Unlock()

	// Send shutdown message to all clients
	shutdownMsg := map[string]interface{}{
		"type":    "system_shutdown",
		"message": "Server is shutting down. Please reconnect later.",
		"timestamp": time.Now(),
	}

	msgBytes, _ := json.Marshal(shutdownMsg)

	for conn, client := range ws.clients {
		conn.WriteMessage(websocket.TextMessage, msgBytes)
		conn.Close()
		delete(ws.clients, conn)
		log.Printf("🔵 Disconnected client UserID: %d", client.UserID)
	}

	log.Println("✅ WebSocket service shutdown complete")
}