package models

type ServiceLog struct {
	Base
	Level   string `gorm:"not null;index" json:"level"`
	Service string `gorm:"not null;index" json:"service"`
	Message string `gorm:"not null"       json:"message"`
}
