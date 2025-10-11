package database

import (
	"log"

	"delivery-management-system/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations...")
	
	err := db.AutoMigrate(
		&models.User{},
		&models.Order{},
	)
	
	if err != nil {
		return err
	}
	
	log.Println("Database migrations completed successfully")
	return nil
}