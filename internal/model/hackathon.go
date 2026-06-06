package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	HackathonPrizeNone         = "none"
	HackathonPrizeCash         = "cash"
	HackathonPrizeNonMonetary  = "non_monetary"
	HackathonRegAuto           = "auto"
	HackathonRegManual         = "manual"
	HackathonTaskAtStart       = "at_start"
	HackathonTaskInDescription = "in_description"
	HackathonWinnerAuto        = "auto"
	HackathonWinnerManual      = "manual"
	RegStatusPending           = "pending"
	RegStatusApproved          = "approved"
	RegStatusRejected          = "rejected"
)

type Hackathon struct {
	ID                   uuid.UUID
	OrganizerID          uuid.UUID
	TitleEnc             []byte
	DescriptionEnc       []byte
	RulesEnc             []byte
	Theme                string
	Format               string
	PrizePoolKZT         float64
	PrizeType            string
	PrizeBreakdown       json.RawMessage
	MinParticipants      int
	MaxParticipants      int
	TeamMinSize          int
	TeamMaxSize          int
	StartsAt             time.Time
	EndsAt               time.Time
	RegistrationOpensAt  *time.Time
	RegistrationDeadline time.Time
	ResultsAnnouncedAt   *time.Time
	RegistrationMode     string
	TaskReveal           string
	TaskBodyEnc          []byte
	SubmissionSchema     json.RawMessage
	BlindJudging         bool
	WinnerMode           string
	CatalogPriority      int
	PublicSubmissions    bool
	PrizeEscrowRecorded  bool
	ResultsLocked        bool
	ListingFeePaid       bool
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type HackathonUpdateInput struct {
	Title                string
	Description          string
	Rules                string
	Theme                string
	Format               string
	PrizePoolKZT         float64
	PrizeType            string
	PrizeBreakdown       json.RawMessage
	MinParticipants      int
	MaxParticipants      int
	TeamMinSize          int
	TeamMaxSize          int
	StartsAt             time.Time
	EndsAt               time.Time
	RegistrationOpensAt  time.Time
	RegistrationDeadline time.Time
	ResultsAnnouncedAt   time.Time
	RegistrationMode     string
	TaskReveal           string
	TaskBody             string
	SubmissionSchema     json.RawMessage
	BlindJudging         bool
	WinnerMode           string
	PublicSubmissions    bool
}

type HackathonTeam struct {
	ID          uuid.UUID
	HackathonID uuid.UUID
	Name        string
	CaptainID   uuid.UUID
	InviteCode  string
	Recruiting  bool
	CreatedAt   time.Time
}

type HackathonRegistration struct {
	ID          uuid.UUID
	HackathonID uuid.UUID
	StudentID   uuid.UUID
	TeamID      *uuid.UUID
	Status      string
	CreatedAt   time.Time
}

type SubmissionPayload struct {
	Description     string
	ArtifactKey     string
	PresentationKey string
	RepoURL         string
	VideoURL        string
}

type HackathonSubmission struct {
	ID              uuid.UUID
	HackathonID     uuid.UUID
	TeamID          *uuid.UUID
	StudentID       *uuid.UUID
	Description     string
	ArtifactKey     *string
	PresentationKey string
	RepoURL         string
	VideoURL        string
	VersionNo       int
	SubmittedAt     time.Time
}

type HackathonSubmissionVersion struct {
	ID              uuid.UUID
	SubmissionID    uuid.UUID
	VersionNo       int
	Description     string
	ArtifactKey     string
	PresentationKey string
	RepoURL         string
	VideoURL        string
	CreatedAt       time.Time
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
	CertType       string
	Place          int
	CreatedAt      time.Time
}

type HackathonAnnouncement struct {
	ID          uuid.UUID
	HackathonID uuid.UUID
	Title       string
	Body        string
	CreatedAt   time.Time
}
