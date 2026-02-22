package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uni-intern-marketplace/intern-server/internal/database"
	"github.com/uni-intern-marketplace/intern-server/internal/models"
)

// Өтінім жіберу (Apply)
func ApplyToInternship(c *gin.Context) {
	var input models.Application
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Мәліметтер қате толтырылды"})
		return
	}

	// Базаға сақтау
	if err := database.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Өтінімді сақтау мүмкін болмады"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Өтінім сәтті жіберілді!", "data": input})
}

// Компания өз вакансиясына келген өтінімдерді көруі үшін
func GetApplicationsByInternship(c *gin.Context) {
	id := c.Param("id")
	var apps []models.Application
	database.DB.Where("internship_id = ?", id).Find(&apps)
	c.JSON(http.StatusOK, apps)
}
