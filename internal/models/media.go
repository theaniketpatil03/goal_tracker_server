package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Audio struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"userId" json:"-"`
	Title     string             `bson:"title" json:"title"`
	MimeType  string             `bson:"mimeType" json:"mimeType"`
	FilePath  string             `bson:"filePath" json:"-"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

type Video struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"userId" json:"-"`
	Title     string             `bson:"title" json:"title"`
	MimeType  string             `bson:"mimeType" json:"mimeType"`
	FilePath  string             `bson:"filePath" json:"-"`
	AudioFilePath string        `bson:"audioFilePath,omitempty" json:"-"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

