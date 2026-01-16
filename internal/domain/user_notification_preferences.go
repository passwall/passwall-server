package domain

import "time"

// UserNotificationPreferences stores per-user notification delivery preferences.
//
// This is intentionally a 1:1 table keyed by user_id (PK) to keep reads/updates
// fast and to avoid having to manage separate IDs for a purely user-scoped row.
type UserNotificationPreferences struct {
	UserID uint `json:"user_id" gorm:"primaryKey"`

	CommunicationEmails bool `json:"communication_emails" gorm:"not null;default:false"`
	MarketingEmails     bool `json:"marketing_emails" gorm:"not null;default:false"`
	SocialEmails        bool `json:"social_emails" gorm:"not null;default:false"`

	// Security emails are mandatory and should not be disabled.
	SecurityEmails bool `json:"security_emails" gorm:"not null;default:true"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserNotificationPreferences) TableName() string {
	return "user_notification_preferences"
}

// UserNotificationPreferencesDTO is the API response shape.
type UserNotificationPreferencesDTO struct {
	CommunicationEmails bool `json:"communication_emails"`
	MarketingEmails     bool `json:"marketing_emails"`
	SocialEmails        bool `json:"social_emails"`
	SecurityEmails      bool `json:"security_emails"`
}

func ToUserNotificationPreferencesDTO(p *UserNotificationPreferences) *UserNotificationPreferencesDTO {
	if p == nil {
		return nil
	}

	return &UserNotificationPreferencesDTO{
		CommunicationEmails: p.CommunicationEmails,
		MarketingEmails:     p.MarketingEmails,
		SocialEmails:        p.SocialEmails,
		SecurityEmails:      p.SecurityEmails,
	}
}

// UpdateUserNotificationPreferencesRequest supports partial updates.
// (Booleans are pointers so "unset" can be distinguished from false.)
type UpdateUserNotificationPreferencesRequest struct {
	CommunicationEmails *bool `json:"communication_emails"`
	MarketingEmails     *bool `json:"marketing_emails"`
	SocialEmails        *bool `json:"social_emails"`
	SecurityEmails      *bool `json:"security_emails"`
}
