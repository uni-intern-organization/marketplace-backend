package models

import "gorm.io/gorm"

type Internship struct {
	gorm.Model
	Title       string  `json:"title"`
	Description string  `json:"description"`
	CompanyName string  `json:"company_name"`
	Category    string  `json:"category"`
	Location    string  `json:"location"`
	Salary      float64 `json:"salary"`
	EmployerID  uint    `json:"employer_id"`
}
