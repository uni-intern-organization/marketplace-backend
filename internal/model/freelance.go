package model

import (
	"time"

	"github.com/google/uuid"
)

type FreelanceTask struct {
	ID                 uuid.UUID
	RecruiterID        uuid.UUID
	TitleEnc           []byte
	DescriptionEnc     []byte
	Category           string
	BudgetKZT          float64
	Deadline           time.Time
	RequiredSkills     string
	Status             string
	EscrowStatus       string
	AcceptedStudentID  *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type FreelanceProposal struct {
	ID         uuid.UUID
	TaskID     uuid.UUID
	StudentID  uuid.UUID
	MessageEnc []byte
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type FreelanceSubmission struct {
	ID             uuid.UUID
	TaskID         uuid.UUID
	StudentID      uuid.UUID
	DeliverableKey *string
	StudentNote    string
	RevisionCount  int
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type FreelanceReview struct {
	ID         uuid.UUID
	TaskID     uuid.UUID
	ReviewerID uuid.UUID
	Rating     int
	Comment    string
	CreatedAt  time.Time
}

type FreelanceDispute struct {
	ID         uuid.UUID
	TaskID     uuid.UUID
	OpenedBy   uuid.UUID
	Reason     string
	Resolution string
	Status     string
	ResolvedBy *uuid.UUID
	CreatedAt  time.Time
	ResolvedAt *time.Time
}
