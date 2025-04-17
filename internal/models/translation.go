package models

import "gorm.io/gorm"

// TranslationResponse represents the data stored in the database cache.
type TranslationResponse struct {
	gorm.Model
	Text        string `gorm:"index:idx_keyword,priority:1"`
	Selected    string `gorm:"index:idx_keyword,priority:2"`
	Translation string
}
