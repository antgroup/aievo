package db

import "gorm.io/gorm"

type DBStorageOption func(*Storage)

func WithDB(db *gorm.DB) DBStorageOption {
	return func(s *Storage) {
		s.db = db
	}
}
