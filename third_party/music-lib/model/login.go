package model

type QRLoginStatus string

const (
	QRLoginStatusWaiting QRLoginStatus = "waiting"
	QRLoginStatusScanned QRLoginStatus = "scanned"
	QRLoginStatusSuccess QRLoginStatus = "success"
	QRLoginStatusExpired QRLoginStatus = "expired"
	QRLoginStatusFailed  QRLoginStatus = "failed"
)

type QRLoginSession struct {
	Source    string            `json:"source"`
	Key       string            `json:"key"`
	URL       string            `json:"url"`
	ImageURL  string            `json:"image_url,omitempty"`
	State     string            `json:"state,omitempty"`
	ExpiresAt int64             `json:"expires_at,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

type QRLoginResult struct {
	Source  string            `json:"source"`
	Key     string            `json:"key"`
	Status  QRLoginStatus     `json:"status"`
	Message string            `json:"message,omitempty"`
	Cookie  string            `json:"cookie,omitempty"`
	Cookies map[string]string `json:"cookies,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"`
}
