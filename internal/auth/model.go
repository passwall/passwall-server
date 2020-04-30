package auth

//LoginDTO ...
type LoginDTO struct {
	Username string `validate:"required"`
	Password string `validate:"required"`
}

//TokenDetailsDTO ...
type TokenDetailsDTO struct {
	AccessToken  string
	RefreshToken string
	AtExpires    int64
	RtExpires    int64
}
