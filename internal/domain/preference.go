package domain

import "time"

// Preference stores a single preference value for a scoped owner (user/org).
// One row represents one setting: (owner_type, owner_id, section, key) → (type, value).
type Preference struct {
	ID uint `json:"id" gorm:"primaryKey"`

	OwnerType string `json:"owner_type" gorm:"type:varchar(16);not null;uniqueIndex:uq_preferences_owner_section_key,priority:1;index:idx_preferences_owner_section,priority:1"`
	OwnerID   uint   `json:"owner_id" gorm:"not null;uniqueIndex:uq_preferences_owner_section_key,priority:2;index:idx_preferences_owner_section,priority:2"`

	Section string `json:"section" gorm:"type:varchar(64);not null;uniqueIndex:uq_preferences_owner_section_key,priority:3;index:idx_preferences_owner_section,priority:3"`
	Key     string `json:"key" gorm:"type:varchar(64);not null;uniqueIndex:uq_preferences_owner_section_key,priority:4"`

	// Value is stored as text; interpretation depends on Type.
	// Type can be: string | number | boolean | json
	Value string `json:"value" gorm:"type:text;not null;default:''"`
	Type  string `json:"type" gorm:"type:varchar(16);not null;default:'string'"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Preference) TableName() string {
	return "preferences"
}

type PreferenceDTO struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

func ToPreferenceDTO(p *Preference) *PreferenceDTO {
	if p == nil {
		return nil
	}
	return &PreferenceDTO{
		Section: p.Section,
		Key:     p.Key,
		Type:    p.Type,
		Value:   p.Value,
	}
}

type UpsertPreferenceRequest struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

type UpsertPreferencesRequest struct {
	Preferences []UpsertPreferenceRequest `json:"preferences"`
}
