package models

const (
	APIKeyHeader     = "X-Api-Key"
	APIServiceHeader = "X-Api-Service"

	FieldToken     = "token"
	FieldUserAgent = "userAgent"
	FieldIP        = "ip"
	FieldOrigin    = "origin"
	FieldRole      = "role"
)

type AuthResponse struct {
	UserID string `json:"userId"`
	TTL    int64  `json:"ttl"` // time duration in second, -1 for infinite

	// Policies map[string]Policy `json:"policies,omitempty"`
}

type AuthRequest struct {
	SessionID int64  `json:"session_id"`
	Token     string `json:"token"`
	UserAgent string `json:"userAgent"`
	IP        string `json:"ip"`
	Origin    string `json:"origin"`
	Role      string `json:"role"`
}

func NewAuthRequest(data map[string]string) *AuthRequest {
	return &AuthRequest{
		Token:     data[FieldToken],
		UserAgent: data[FieldUserAgent],
		IP:        data[FieldIP],
		Origin:    data[FieldOrigin],
		Role:      data[FieldRole],
	}
}

type Policy struct {
	Allowed bool
}

type SessionInfo struct {
	ID        int64  `json:"session_id"` // ID is primary identifier of session
	UserID    string `json:"user_id"`    // UserID is optional identifier, when client if logged-in
	Token     string `json:"token"`
	UserAgent string `json:"userAgent"`
	IP        string `json:"ip"`
	Origin    string `json:"origin"`
	Role      string `json:"role"`
	TTL       int64  `json:"ttl"`
}
