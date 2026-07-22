package models

type Collection struct {
	Base
	UserID      string `gorm:"type:varchar(36);not null;index" json:"user_id"`
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
}

type CollectionItem struct {
	Base
	CollectionID string `gorm:"type:varchar(36);not null;index:idx_col_item_unique,unique,priority:1" json:"collection_id"`
	MediaType    string `gorm:"not null;index:idx_col_item_unique,unique,priority:2" json:"media_type"` // "movie" | "tvshow"
	MediaID      string `gorm:"type:varchar(36);not null;index:idx_col_item_unique,unique,priority:3" json:"media_id"`
	SortOrder    int    `gorm:"default:0" json:"sort_order"`
}
