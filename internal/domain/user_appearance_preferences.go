package domain

import "time"

// UserAppearancePreferences stores per-user UI preferences (theme, font).
//
// This is a 1:1 table keyed by user_id (PK) to keep reads/updates fast.
type UserAppearancePreferences struct {
	UserID uint `json:"user_id" gorm:"primaryKey"`

	// Theme can be: dark | light | system
	Theme string `json:"theme" gorm:"not null;default:dark"`

	// Font is a UI hint (e.g. inter, manrope, system).
	Font string `json:"font" gorm:"not null;default:inter"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserAppearancePreferences) TableName() string {
	return "user_appearance_preferences"
}

// UserAppearancePreferencesDTO is the API response shape.
type UserAppearancePreferencesDTO struct {
	Theme string `json:"theme"`
	Font  string `json:"font"`
}

func ToUserAppearancePreferencesDTO(p *UserAppearancePreferences) *UserAppearancePreferencesDTO {
	if p == nil {
		return nil
	}

	return &UserAppearancePreferencesDTO{
		Theme: p.Theme,
		Font:  p.Font,
	}
}

// UpdateUserAppearancePreferencesRequest supports partial updates.
// (Fields are pointers so "unset" can be distinguished from empty values.)
type UpdateUserAppearancePreferencesRequest struct {
	Theme *string `json:"theme"`
	Font  *string `json:"font"`
}
