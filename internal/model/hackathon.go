package model

import (
	"time"

	"github.com/google/uuid"
)

type Hackathon struct {
	ID                   uuid.UUID
	OrganizerID          uuid.UUID
	TitleEnc             []byte
	DescriptionEnc       []byte
	Theme                string
	Format               string
	PrizePoolKZT         float64
	MinParticipants      int
	MaxParticipants      int
	StartsAt             time.Time
	EndsAt               time.Time
	RegistrationDeadline time.Time
	ListingFeePaid       bool
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type HackathonTeam struct {
	ID          uuid.UUID
	HackathonID uuid.UUID
	Name        string
	CaptainID   uuid.UUID
	InviteCode  string
	CreatedAt   time.Time
}

type HackathonRegistration struct {
	ID          uuid.UUID
	HackathonID uuid.UUID
	StudentID   uuid.UUID
	TeamID      *uuid.UUID
	CreatedAt   time.Time
}

type HackathonSubmission struct {
	ID          uuid.UUID
	HackathonID uuid.UUID
	TeamID      *uuid.UUID
	StudentID   *uuid.UUID
	ArtifactKey *string
	SubmittedAt time.Time
}

type HackathonResult struct {
	ID              uuid.UUID
	HackathonID     uuid.UUID
	TeamID          *uuid.UUID
	StudentID       *uuid.UUID
	Place           int
	PrizeAmountKZT  float64
	InternshipOffer bool
	CreatedAt       time.Time
}

type HackathonCertificate struct {
	ID             uuid.UUID
	HackathonID    uuid.UUID
	StudentID      uuid.UUID
	CertificateURL string
	CreatedAt      time.Time
}

type HackathonAnnouncement struct {
	ID          uuid.UUID
	HackathonID uuid.UUID
	Title       string
	Body        string
	CreatedAt   time.Time
}
