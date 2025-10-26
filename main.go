package main

import (
	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
)

func main() {
	db.Connect()

	err := db.DB.AutoMigrate(
		&models.City{},
		&models.User{},
		&models.Habit{},
		&models.HabitLog{},
		&models.Achievement{},
	)
	if err != nil {
		return
	}
}
