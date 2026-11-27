package auth

import "time"

type Role string

const (
	RoleStudent       Role = "STUDENT"
	RoleParent        Role = "PARENT"
	RoleTeacher       Role = "TEACHER"
	RoleSchoolAdmin   Role = "SCHOOL_ADMIN"
	RoleAdmin         Role = "ADMIN"
	RoleContentEditor Role = "CONTENT_EDITOR"
)

// Principal is the authenticated identity stored inside JWT claims & request context.
type Principal struct {
	ID              string `json:"id"`
	Role            Role   `json:"role"`
	DisplayName     string `json:"displayName"`
	Grade           int    `json:"grade,omitempty"`
	PreferredLocale string `json:"preferredLocale"`
	Timezone        string `json:"timezone"`
	OTPEnabled      bool   `json:"otpEnabled,omitempty"`
	AvatarItemID    string `json:"avatarItemId,omitempty"`
	TotalSP         int64  `json:"totalSp"`
	AstroCoins      int64  `json:"astroCoins"`
	PilotLevel      int    `json:"pilotLevel,omitempty"`
	CurrentStreak   int    `json:"currentStreak"`
}

// CredentialRecord is returned by Store lookups during password verification.
type CredentialRecord struct {
	Principal    Principal
	PasswordHash string
}

// LiveFeedItem is a strictly privacy-safe public event for the login ticker (zero PII).
type LiveFeedItem struct {
	ID          string    `json:"id"`
	MaskedName  string    `json:"maskedName"`
	Grade       int       `json:"grade"`
	AvatarID    string    `json:"avatarId"`
	Points      int64     `json:"points"`
	SourceLabel string    `json:"sourceLabel"`
	SubmittedAt time.Time `json:"submittedAt"`
}
