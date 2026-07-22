package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type UserRepository interface {
	Count() (int64, error)
	Create(user *models.User) error
	FindByUsername(username string) (*models.User, error)
	FindByID(id string) (*models.User, error)
	List() ([]models.User, error)
	Update(user *models.User) error
	UpdatePassword(userID string, hash string) error
	Delete(userID string) error
}

type userRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepository{db} }

func (r *userRepository) Count() (int64, error) {
	var n int64
	return n, r.db.Model(&models.User{}).Count(&n).Error
}

func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindByUsername(username string) (*models.User, error) {
	var u models.User
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByID(id string) (*models.User, error) {
	var u models.User
	if err := r.db.First(&u, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) List() ([]models.User, error) {
	var users []models.User
	return users, r.db.Order("created_at ASC").Find(&users).Error
}

func (r *userRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) UpdatePassword(userID string, hash string) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", hash).Error
}

func (r *userRepository) Delete(userID string) error {
	result := r.db.Delete(&models.User{}, "id = ?", userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
