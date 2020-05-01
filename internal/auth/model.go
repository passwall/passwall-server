package auth

//LoginDTO ...
type LoginDTO struct {
	Username string `validate:"required" json:"username"`
	Password string `validate:"required" json:"password"`
}

//TokenDetailsDTO ...
type TokenDetailsDTO struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AtExpires    int64  `json:"at_expires"`
	RtExpires    int64  `json:"rt_expires"`
}

type RestoreDTO struct {
	Name string `json:"name"`
}
