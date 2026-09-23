package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type WebhookRepository interface {
	Create(w *models.Webhook) error
	List() ([]models.Webhook, error)
	// ListEnabled returns only enabled webhooks — the set river-events fans
	// deliveries out to.
	ListEnabled() ([]models.Webhook, error)
	FindByID(id string) (*models.Webhook, error)
	Update(w *models.Webhook) error
	Delete(id string) error
	// RecordDelivery persists a delivery outcome and stamps the webhook's
	// last_delivery_at / last_error for an at-a-glance health view.
	RecordDelivery(d *models.WebhookDelivery, lastError string) error
	ListDeliveries(webhookID string, limit int) ([]models.WebhookDelivery, error)
}

type gormWebhookRepository struct{ db *gorm.DB }

func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &gormWebhookRepository{db: db}
}

func (r *gormWebhookRepository) Create(w *models.Webhook) error {
	// Select("*") forces every column to be written, so an explicit Enabled:false
	// isn't silently overridden by the column's default:true (GORM omits Go
	// zero-values otherwise). Base's PK/timestamps are still handled by GORM.
	return r.db.Select("*").Create(w).Error
}

func (r *gormWebhookRepository) List() ([]models.Webhook, error) {
	var out []models.Webhook
	return out, r.db.Order("name").Find(&out).Error
}

func (r *gormWebhookRepository) ListEnabled() ([]models.Webhook, error) {
	var out []models.Webhook
	return out, r.db.Where("enabled = ?", true).Order("name").Find(&out).Error
}

func (r *gormWebhookRepository) FindByID(id string) (*models.Webhook, error) {
	var w models.Webhook
	if err := r.db.First(&w, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *gormWebhookRepository) Update(w *models.Webhook) error {
	return r.db.Save(w).Error
}

func (r *gormWebhookRepository) Delete(id string) error {
	res := r.db.Where("id = ?", id).Delete(&models.Webhook{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *gormWebhookRepository) RecordDelivery(d *models.WebhookDelivery, lastError string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(d).Error; err != nil {
			return err
		}
		return tx.Model(&models.Webhook{}).Where("id = ?", d.WebhookID).
			Updates(map[string]any{"last_delivery_at": d.CreatedAt, "last_error": lastError}).Error
	})
}

func (r *gormWebhookRepository) ListDeliveries(webhookID string, limit int) ([]models.WebhookDelivery, error) {
	var out []models.WebhookDelivery
	q := r.db.Where("webhook_id = ?", webhookID).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return out, q.Find(&out).Error
}
