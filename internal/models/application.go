package models

import "gorm.io/gorm"

type Application struct {
	gorm.Model
	StudentID    uint   `json:"student_id"`
	InternshipID uint   `json:"internship_id"`
	ResumeURL    string `json:"resume_url"`                      // Студенттің резюмесіне сілтеме
	Status       string `json:"status" gorm:"default:'pending'"` // pending, accepted, rejected
	Message      string `json:"message"`                         // Студенттің ілеспе хаты
}
