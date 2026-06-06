package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleStudent   UserRole = "student"
	RoleRecruiter UserRole = "recruiter"
	RoleModerator UserRole = "moderator"
	RoleAdmin     UserRole = "admin"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Role         UserRole
	IsBlocked    bool
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
	AvatarObjectKey *string
	Skills          string // comma-separated for matching
	Education       string
	ExperienceYears int
	Location        string
	Availability    string // remote, hybrid, onsite
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type RecruiterPlan string

const (
	RecruiterPlanFree      RecruiterPlan = "free"
	RecruiterPlanPro       RecruiterPlan = "pro"
	RecruiterPlanStarter   RecruiterPlan = "starter"
	RecruiterPlanBusiness  RecruiterPlan = "business"
	RecruiterPlanCorporate RecruiterPlan = "corporate"
)

type OrganizerType string

const (
	OrganizerCompany    OrganizerType = "company"
	OrganizerUniversity OrganizerType = "university"
)

type RecruiterProfile struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	CompanyNameEnc       []byte
	FullNameEnc          []byte
	PhoneEnc             []byte
	CompanyLogoObjectKey *string
	Plan                 RecruiterPlan
	PlanExpiresAt        *time.Time
	OrganizerType        OrganizerType
	PublicationsQuota    int
	PublicationsUsed     int
	QuotaResetAt         *time.Time
	InvitationsQuota     int
	InvitationsUsed      int
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
	ID                   uuid.UUID
	StudentID            uuid.UUID
	RecruiterID          uuid.UUID
	VacancyID            *uuid.UUID
	InvitationID         *uuid.UUID
	CoverLetterEnc       []byte
	Status               string
	InterviewFormat      string
	InterviewMessage     string
	ProposedSlots        []time.Time
	InterviewScheduledAt *time.Time
	DecisionReason       string
	OfferStartDate       *time.Time
	OfferTerms           string
	OfferDuration        string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ListingTier string

const (
	ListingTierBasic    ListingTier = "basic"
	ListingTierStandard ListingTier = "standard"
	ListingTierPremium  ListingTier = "premium"
)

type VacancyStatus string

const (
	VacancyStatusDraft         VacancyStatus = "draft"
	VacancyStatusPendingReview VacancyStatus = "pending_review"
	VacancyStatusNeedsRevision VacancyStatus = "needs_revision"
	VacancyStatusActive        VacancyStatus = "active"
	VacancyStatusArchived      VacancyStatus = "archived"
	VacancyStatusRejected      VacancyStatus = "rejected"
)

type Vacancy struct {
	ID                 uuid.UUID
	RecruiterID        uuid.UUID
	TitleEnc           []byte
	DescriptionEnc     []byte
	CompanyName        string
	RequiredSkills     string
	Location           string
	EmploymentType     string
	MinExperienceYears int
	ListingTier        ListingTier
	PublishedAt        *time.Time
	ExpiresAt          *time.Time
	Status             VacancyStatus
	IsFeatured         bool
	FeaturedUntil      *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type BillingEvent struct {
	ID          uuid.UUID
	RecruiterID uuid.UUID
	EventType   string
	Metadata    []byte
	CreatedAt   time.Time
}
