package database

import (
	"context"
	"fmt"
	"log"

	"github.com/zzhirong/contextdict/config"          // Adjust import path if needed
	"github.com/zzhirong/contextdict/internal/models" // Adjust import path
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Repository defines the interface for database operations.
type Repository interface {
	FindTranslation(ctx context.Context, text, selected string) (*models.TranslationResponse, error)
	CreateTranslation(ctx context.Context, record *models.TranslationResponse) error
	Close() error
}

// GormRepository implements the Repository interface using GORM.
type GormRepository struct {
	db *gorm.DB
}

// NewRepository creates a new database connection and repository instance.
func NewRepository(cfg config.DatabaseConfig) (Repository, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Database connection established.")

	// Auto-migrate the schema
	if err = db.AutoMigrate(&models.TranslationResponse{}); err != nil {
		// Attempt to close DB if migration fails
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, fmt.Errorf("failed to auto-migrate database schema: %w", err)
	}
	log.Println("Database schema migrated.")

	return &GormRepository{db: db}, nil
}

// FindTranslation looks for an existing translation in the cache.
func (r *GormRepository) FindTranslation(ctx context.Context, text, selected string) (*models.TranslationResponse, error) {
	tr := otel.Tracer("database")
	ctx, span := tr.Start(ctx, "FindTranslation")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "mysql"),
		attribute.String("db.statement.type", "SELECT"),
		attribute.String("translation.text", text),
		attribute.String("translation.selected", selected),
	)
	var result models.TranslationResponse
	result.Text = text
	result.Selected = selected
	err := r.db.WithContext(ctx).
		Where(&result).
		First(&result).Error

	if err != nil {
		span.RecordError(err)
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Indicate cache miss clearly
		}
		// For other errors, return the error.
		return nil, fmt.Errorf("error finding translation in DB: %w", err)
	}
	return &result, nil
}

// CreateTranslation saves a new translation record to the cache.
func (r *GormRepository) CreateTranslation(ctx context.Context, record *models.TranslationResponse) error {
	tr := otel.Tracer("database")
	ctx, span := tr.Start(ctx, "CreateTranslation")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "mysql"),
		attribute.String("db.statement.type", "INSERT"),
		attribute.String("translation.text", record.Text),
		attribute.String("translation.selected", record.Selected),
	)
	// Use .WithContext for potential cancellation/timeouts
	// Ensure we don't try to insert a record with an existing primary key if it came from FindTranslation
	if record.ID != 0 {
		record.ID = 0 // Reset ID to ensure GORM creates a new record
	}

	err := r.db.WithContext(ctx).Create(record).Error
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("error creating translation in DB: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (r *GormRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying *sql.DB: %w", err)
	}
	log.Println("Closing database connection.")
	return sqlDB.Close()
}
