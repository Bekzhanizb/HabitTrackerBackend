package models

import "time"

type City struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"unique" json:"name"`
}

type User struct {
	ID           uint          `gorm:"primaryKey" json:"id"`
	Username     string        `gorm:"unique" json:"username"`
	PasswordHash string        `json:"password_hash"`
	CityID       *uint         `json:"city_id"`
	City         City          `gorm:"foreignKey:CityID"`
	Role         string        `gorm:"default:user" json:"role"`
	Picture      string        `gorm:"default:'/uploads/default.png'" json:"picture"`
	CreatedAt    time.Time     `gorm:"autoCreateTime" json:"created_at"`
	Habits       []Habit       `gorm:"foreignKey:UserID"`
	Achievements []Achievement `gorm:"foreignKey:UserID"`
}

type Habit struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `json:"user_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Frequency   string     `json:"frequency"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	Logs        []HabitLog `gorm:"foreignKey:HabitID"`
}

type HabitLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	HabitID     uint      `json:"habit_id"`
	Date        time.Time `json:"date"`
	IsCompleted bool      `gorm:"default:false" json:"is_completed"`
}

type Achievement struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	EarnedAt    time.Time `gorm:"autoCreateTime" json:"earned_at"`
}
