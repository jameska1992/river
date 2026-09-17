package models

type ServiceLog struct {
	Base
	Level   string `gorm:"not null;index" json:"level"`
	Service string `gorm:"not null;index" json:"service"`
	Message string `gorm:"not null"       json:"message"`
	// CreatedBy is the authenticated principal that wrote the entry (the
	// service account or an admin username), stamped server-side from the JWT
	// — not taken from the request body. `Service` above is a self-declared
	// component label (the internal services share one account), so CreatedBy
	// is the real attribution and exposes any spoofed Service value.
	CreatedBy string `gorm:"index" json:"created_by"`
}
