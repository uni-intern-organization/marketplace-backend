package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	AppStatusNew                    = "new"
	AppStatusViewed                 = "viewed"
	AppStatusUnderReview            = "under_review"
	AppStatusInvited                = "interview_invited"
	AppStatusInterviewScheduled     = "interview_scheduled"
	AppStatusAwaitingDecision       = "awaiting_decision"
	AppStatusOfferSent              = "offer_sent"
	AppStatusHired                  = "hired"
	AppStatusRejected               = "rejected"
	AppStatusRejectedAfterInterview = "rejected_after_interview"
	AppStatusAccepted               = "accepted"
	AppStatusCompleted              = "completed"
)

type ModerationReview struct {
	ID          uuid.UUID
	EntityType  string
	EntityID    uuid.UUID
	ModeratorID uuid.UUID
	Action      string
	Reason      string
	Comment     string
	CreatedAt   time.Time
}

type AuditEntry struct {
	ID         uuid.UUID
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Metadata   []byte
	CreatedAt  time.Time
}

type RecruiterVerification struct {
	ID          uuid.UUID
	RecruiterID uuid.UUID
	BIN         string
	DocumentKey *string
	Status      string
	ReviewedBy  *uuid.UUID
	Comment     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Conversation struct {
	ID           uuid.UUID
	StudentID    uuid.UUID
	RecruiterID  uuid.UUID
	ContextType  string
	ContextID    uuid.UUID
	ContextTitle string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	BodyEnc        []byte
	AttachmentKey  *string
	ReadAt         *time.Time
	CreatedAt      time.Time
}

type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Type      string
	Title     string
	Body      string
	Payload   []byte
	ReadAt    *time.Time
	CreatedAt time.Time
}

type NotificationPreference struct {
	UserID           uuid.UUID
	NotificationType string
	ChannelInApp     bool
	ChannelEmail     bool
	ChannelPush      bool
}

type WalletTransaction struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	AmountKZT     float64
	Type          string
	ReferenceType string
	ReferenceID   *uuid.UUID
	Status        string
	CreatedAt     time.Time
}

type WithdrawalRequest struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	AmountKZT   float64
	CardLast4   string
	Status      string
	ProcessedBy *uuid.UUID
	CreatedAt   time.Time
	ProcessedAt *time.Time
}

type PromoCode struct {
	ID              uuid.UUID
	Code            string
	DiscountPercent int
	MaxUses         int
	UsesCount       int
	ExpiresAt       *time.Time
	CreatedAt       time.Time
}

type ProfileSectionVisibility struct {
	UserID               uuid.UUID
	SkillsVisibility     string
	EducationVisibility  string
	ExperienceVisibility string
	PortfolioVisibility  string
	HackathonsVisibility string
	ReviewsVisibility    string
}

type ProfileCompletion struct {
	Percent int      `json:"percent"`
	Badge   string   `json:"badge,omitempty"`
	Missing []string `json:"missing"`
}

type UserComplaint struct {
	ID         uuid.UUID
	ReporterID uuid.UUID
	TargetType string
	TargetID   uuid.UUID
	Reason     string
	Details    string
	Status     string
	ReviewedBy *uuid.UUID
	Resolution string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

type StaffTask struct {
	ID          uuid.UUID
	CreatedBy   uuid.UUID
	AssignedTo  *uuid.UUID
	Title       string
	Description string
	EntityType  string
	EntityID    *uuid.UUID
	Status      string
	Resolution  string
	ResolvedBy  *uuid.UUID
	CreatedAt   time.Time
	ResolvedAt  *time.Time
}
