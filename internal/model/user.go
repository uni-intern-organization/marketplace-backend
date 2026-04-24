package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleStudent   UserRole = "student"
	RoleRecruiter UserRole = "recruiter"
	RoleAdmin     UserRole = "admin"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Role         UserRole
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type StudentProfile struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	FullNameEnc     []byte
	PhoneEnc        []byte
	BioEnc          []byte
	ResumeObjectKey *string
	Skills          string // comma-separated for matching
	Education       string
	ExperienceYears int
	Location        string
	Availability    string // remote, hybrid, onsite
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type RecruiterProfile struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	CompanyNameEnc       []byte
	FullNameEnc          []byte
	PhoneEnc             []byte
	CompanyLogoObjectKey *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Invitation struct {
	ID          uuid.UUID
	RecruiterID uuid.UUID
	StudentID   uuid.UUID
	MessageEnc  []byte
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Application struct {
	ID             uuid.UUID
	StudentID      uuid.UUID
	RecruiterID    uuid.UUID
	VacancyID      *uuid.UUID
	InvitationID   *uuid.UUID
	CoverLetterEnc []byte
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Vacancy struct {
	ID                 uuid.UUID
	RecruiterID        uuid.UUID
	TitleEnc           []byte
	DescriptionEnc     []byte
	CompanyName        string // company name for the vacancy
	RequiredSkills     string // comma-separated
	Location           string
	EmploymentType     string // remote, hybrid, onsite
	MinExperienceYears int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
