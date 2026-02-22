package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uni-intern-marketplace/intern-server/internal/database"
	"github.com/uni-intern-marketplace/intern-server/internal/models"
)

// Барлық стажировкаларды алу
func GetInternships(c *gin.Context) {
	var internships []models.Internship
	database.DB.Find(&internships)
	c.JSON(http.StatusOK, internships)
}

// Жаңа стажировка қосу (Компания үшін)
func CreateInternship(c *gin.Context) {
	var input models.Internship
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Create(&input)
	c.JSON(http.StatusCreated, input)
}
