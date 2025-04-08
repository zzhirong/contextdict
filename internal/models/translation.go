package models

import "gorm.io/gorm"

// TranslationResponse represents the data stored in the database cache.
type TranslationResponse struct {
	gorm.Model
	Text        string `gorm:"index:idx_keyword_text"`
	Selected    string `gorm:"idx_keyword_selected"`
	Translation string
}
