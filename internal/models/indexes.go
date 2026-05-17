package models

import (
	"context"
	"time"

	"goal_tracker_server/internal/config"
	mongoutils "goal_tracker_server/internal/mongo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func EnsureIndexes(mongoClient *mongo.Client, cfg config.Config) error {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	db := mongoClient.Database(dbName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Users (email unique).
	users := db.Collection("users")
	_, _ = users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("users_email_unique"),
	})

	// Audios/Videos.
	audios := db.Collection("audios")
	_, _ = audios.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
		Options: options.Index().SetName("audios_userId_createdAt"),
	})
	videos := db.Collection("videos")
	_, _ = videos.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
		Options: options.Index().SetName("videos_userId_createdAt"),
	})

	// Rewards.
	rewards := db.Collection("rewards")
	_, _ = rewards.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
		Options: options.Index().SetName("rewards_userId_createdAt"),
	})

	// Quotes.
	quotes := db.Collection("quotes")
	_, _ = quotes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
		Options: options.Index().SetName("quotes_userId_createdAt"),
	})

	// Tasks.
	tasks := db.Collection("tasks")
	_, _ = tasks.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "status", Value: 1}, {Key: "endAt", Value: 1}},
		Options: options.Index().SetName("tasks_userId_status_endAt"),
	})
	_, _ = tasks.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "priority", Value: 1}, {Key: "endAt", Value: 1}},
		Options: options.Index().SetName("tasks_userId_priority_endAt"),
	})

	return nil
}

