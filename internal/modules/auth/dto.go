package auth

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=255"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name" validate:"required,min=2,max=255"`
	Email string `json:"email" validate:"required,email"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	Password        string `json:"password" validate:"required,min=8"`
}

type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresIn    int          `json:"expires_in"`
}

type UserResponse struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Email           string        `json:"email"`
	EmailVerifiedAt *string       `json:"email_verified_at,omitempty"`
	CurrentTeamID   *string       `json:"current_team_id,omitempty"`
	CurrentTeam     *TeamResponse `json:"current_team,omitempty"`
	CreatedAt       string        `json:"created_at"`
}

type TeamResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	OwnerID      string `json:"owner_id"`
	PersonalTeam bool   `json:"personal_team"`
}

func ToUserResponse(user *User) UserResponse {
	resp := UserResponse{
		ID:            user.ID,
		Name:          user.Name,
		Email:         user.Email,
		CurrentTeamID: user.CurrentTeamID,
		CreatedAt:     user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if user.EmailVerifiedAt != nil {
		verified := user.EmailVerifiedAt.Format("2006-01-02T15:04:05Z")
		resp.EmailVerifiedAt = &verified
	}

	if user.CurrentTeam != nil {
		resp.CurrentTeam = &TeamResponse{
			ID:           user.CurrentTeam.ID,
			Name:         user.CurrentTeam.Name,
			OwnerID:      user.CurrentTeam.OwnerID,
			PersonalTeam: user.CurrentTeam.PersonalTeam,
		}
	}

	return resp
}
