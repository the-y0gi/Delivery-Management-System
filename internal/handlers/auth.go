package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"delivery-management-system/internal/auth"
	"delivery-management-system/internal/models"
	"delivery-management-system/internal/utils"
)

type AuthHandler struct {
	DB *gorm.DB
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// Register a new user and return JWT token
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := h.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		utils.SendError(c, http.StatusConflict, "User already exists with this email")
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Create user
	user := models.User{
		Email:     req.Email,
		Password:  hashedPassword,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      models.RoleCustomer, // Default role
	}

	if err := h.DB.Create(&user).Error; err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email, string(user.Role))
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Don't send password in response
	user.Password = ""

	utils.SendSuccess(c, http.StatusCreated, "User registered successfully", AuthResponse{
		Token: token,
		User:  user,
	})
}

//login user and return JWT token
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Find user by email
	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.SendError(c, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check password
	if !auth.CheckPasswordHash(req.Password, user.Password) {
		utils.SendError(c, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email, string(user.Role))
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Don't send password in response
	user.Password = ""

	utils.SendSuccess(c, http.StatusOK, "Login successful", AuthResponse{
		Token: token,
		User:  user,
	})
}

// GetProfile returns the profile of the authenticated user
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		utils.SendError(c, http.StatusNotFound, "User not found")
		return
	}

	// Don't send password
	user.Password = ""

	utils.SendSuccess(c, http.StatusOK, "Profile retrieved successfully", user)
}