package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TaskStatus is kept as string in Mongo for easier filtering and future extension.
type TaskStatus string

const (
	TaskStatusNotStarted TaskStatus = "not_started"
	TaskStatusCompleted  TaskStatus = "completed"
)

// TaskGoal determines how the task should be scheduled/considered in the app.
type TaskGoal string

const (
	TaskGoalDaily   TaskGoal = "daily"
	TaskGoalWeekly  TaskGoal = "weekly"
	TaskGoalMonthly TaskGoal = "monthly"
	TaskGoalYearly  TaskGoal = "yearly"
)

type Task struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      primitive.ObjectID `bson:"userId" json:"-"`

	Name        string     `bson:"name" json:"name"`
	Description string     `bson:"description" json:"description"`
	Status      TaskStatus `bson:"status" json:"status"`

	EndAt    time.Time `bson:"endAt" json:"endAt"`
	Priority string    `bson:"priority" json:"priority"`
	Why      string    `bson:"why" json:"why"`

	// RewardId is selected after completion. Null/empty allowed for not-completed tasks.
	RewardID *primitive.ObjectID `bson:"rewardId,omitempty" json:"rewardId"`

	Goal TaskGoal `bson:"goal" json:"goal"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

