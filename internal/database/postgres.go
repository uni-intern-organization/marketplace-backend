package database

import (
	"log"

	"github.com/uni-intern-marketplace/intern-server/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase(dsn string) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Авто-миграция (Кестелерді автоматты жасау)
	db.AutoMigrate(&models.User{}, &models.Internship{})

	DB = db
}
