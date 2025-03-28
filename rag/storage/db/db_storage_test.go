package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/antgroup/aievo/rag"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestGormDB() (*gorm.DB, error) {
	dsn := os.Getenv("AIEVO_DSN")
	var err error
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, _ := db.DB()
	if err = sqlDB.Ping(); err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Minute * 1000)
	return db, nil
}

func TestLoad(t *testing.T) {
	db, err := newTestGormDB()
	if err != nil {
		t.Fatal(err)
	}

	wfCtx := rag.NewWorkflowContext()
	wfCtx.Id = 10

	storage := NewStorage(WithDB(db))
	ctx := context.Background()
	err = storage.Load(ctx, wfCtx)
	if err != nil {
		t.Fatal(err)
	}
}
