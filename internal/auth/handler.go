package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uni-intern-marketplace/intern-server/internal/database"
	"github.com/uni-intern-marketplace/intern-server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// Тіркелу
func Register(c *gin.Context) {
	var input models.User
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Парольді хэштеу (қауіпсіздік)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 14)
	input.Password = string(hashedPassword)

	if err := database.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Бұл Email бос емес"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Тіркелу сәтті аяқталды"})
}
