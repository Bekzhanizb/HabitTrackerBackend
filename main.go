package main

import (
	"github.com/Bekzhanizb/TaskManager/db"
	"github.com/Bekzhanizb/TaskManager/models"
)

func main() {
	database.Connect()

	database.DB.AutoMigrate(
		&models.City{},
		&models.User{},
		&models.Habit{},
		&models.HabitLog{},
		&models.Achievement{},
	)
}
