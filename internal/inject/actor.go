package inject

// Actor is injected into the page for optimistic local avatar paint.
type Actor struct {
	UserID    string `json:"userId"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}
