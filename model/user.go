package model

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// User model should be something like this
type User struct {
	ID               uint       `gorm:"primary_key" json:"id"`
	UUID             uuid.UUID  `gorm:"type:uuid; type:varchar(100);"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at"`
	Name             string     `json:"name"`
	Email            string     `json:"email"`
	MasterPassword   string     `json:"master_password"`
	Secret           string     `json:"secret"`
	Schema           string     `json:"schema"`
	Role             string     `json:"role"`
	ConfirmationCode string     `json:"confirmation_code"`
	EmailVerifiedAt  time.Time  `json:"email_verified_at"`
}

//UserDTO DTO object for User type
type UserDTO struct {
	ID              uint      `json:"id"`
	UUID            uuid.UUID `json:"uuid"`
	Name            string    `json:"name" validate:"max=100"`
	Email           string    `json:"email" validate:"required,email"`
	MasterPassword  string    `json:"master_password" validate:"required,max=100,min=6"`
	Secret          string    `json:"secret"`
	Schema          string    `json:"schema"`
	Role            string    `json:"role"`
	EmailVerifiedAt time.Time `json:"email_verified_at"`
}

type UserSignup struct {
	Name           string `json:"name" validate:"max=100"`
	Email          string `json:"email" validate:"required,email"`
	MasterPassword string `json:"master_password" validate:"required,max=100,min=6"`
	Recaptcha      string `json:"g_captcha_value" validate:"required"`
}

//UserDTOTable ...
type UserDTOTable struct {
	ID     uint      `json:"id"`
	UUID   uuid.UUID `json:"uuid"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
	Schema string    `json:"schema"`
	Role   string    `json:"role"`
}

func ConvertUserDTO(userSignup *UserSignup) *UserDTO {
	return &UserDTO{
		Name:           userSignup.Name,
		Email:          userSignup.Email,
		MasterPassword: userSignup.MasterPassword,
	}
}

// ToUser ...
func ToUser(userDTO *UserDTO) *User {
	return &User{
		ID:              userDTO.ID,
		UUID:            userDTO.UUID,
		Name:            userDTO.Name,
		Email:           userDTO.Email,
		MasterPassword:  userDTO.MasterPassword,
		Secret:          userDTO.Secret,
		Schema:          userDTO.Schema,
		Role:            userDTO.Role,
		EmailVerifiedAt: userDTO.EmailVerifiedAt,
	}
}

// ToUserDTO ...
func ToUserDTO(user *User) *UserDTO {
	return &UserDTO{
		ID:     user.ID,
		UUID:   user.UUID,
		Name:   user.Name,
		Email:  user.Email,
		Secret: user.Secret,
		Schema: user.Schema,
		Role:   user.Role,
	}
}

// ToUserDTOTable ...
func ToUserDTOTable(user User) UserDTOTable {
	return UserDTOTable{
		ID:     user.ID,
		UUID:   user.UUID,
		Name:   user.Name,
		Email:  user.Email,
		Schema: user.Schema,
		Role:   user.Role,
	}
}

// ToUserDTOs ...
func ToUserDTOs(users []User) []UserDTOTable {
	userDTOs := make([]UserDTOTable, len(users))

	for i, itm := range users {
		userDTOs[i] = ToUserDTOTable(itm)
	}

	return userDTOs
}

/*
{
	"name":	"Erhan Yakut",
	"email": "hello@passwall.io",
	"master_password": "dummypassword",
}
*/
