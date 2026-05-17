package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Quote struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"userId" json:"-"`
	Text      string             `bson:"text" json:"text"`
	Author    string             `bson:"author" json:"author"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

