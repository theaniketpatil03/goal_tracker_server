package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FinanceType string

const (
	FinanceTypeIncome  FinanceType = "income"
	FinanceTypeExpense FinanceType = "expense"
)

type FinanceEntry struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID   primitive.ObjectID `bson:"userId" json:"-"`
	Category string             `bson:"category" json:"category"`
	Type     FinanceType        `bson:"type" json:"type"`
	Amount   float64            `bson:"amount" json:"amount"`
	Time     time.Time          `bson:"time" json:"time"`
	Comment  string             `bson:"comment" json:"comment"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
