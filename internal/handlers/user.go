package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uni-intern-marketplace/intern-server/internal/database"
	"github.com/uni-intern-marketplace/intern-server/internal/models"
)

// Пайдаланушының жеке мәліметтерін алу
func GetProfile(c *gin.Context) {
	// Middleware-ден келген email-ды алу
	email, _ := c.Get("userEmail")

	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пайдаланушы табылмады"})
		return
	}

	c.JSON(http.StatusOK, user)
}
