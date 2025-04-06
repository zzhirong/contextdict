package models

import "gorm.io/gorm"

// TranslationResponse represents the data stored in the database cache.
type TranslationResponse struct {
	gorm.Model
	Keyword     string `gorm:"index:idx_keyword_context" form:"keyword"` // Query param binding
	Context     string `gorm:"idx_keyword_context" form:"context"`   // Query param binding
	Translation string
}
