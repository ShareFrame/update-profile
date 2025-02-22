package models

type UserProfile struct {
	NSID           string `json:"nsid"`
	DisplayName    string `json:"displayName"`
	Bio            string `json:"bio,omitempty"`
	ProfilePicture string `json:"profilePicture,omitempty"`
	ProfileBanner  string `json:"profileBanner,omitempty"`
	Theme          string `json:"theme,omitempty"`
	PrimaryColor   string `json:"primaryColor,omitempty"`
	SecondaryColor string `json:"secondaryColor,omitempty"`
	UpdatedAt      string `json:"updatedAt"`
}

type RequestPayload struct {
	Repo        string      `json:"repo"`
	Profile     UserProfile `json:"profile"`
	BearerToken string      `json:"bearerToken"`
}
