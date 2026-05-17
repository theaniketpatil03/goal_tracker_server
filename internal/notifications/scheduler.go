package notifications

import (
	"context"
	"log"
	"time"

	"goal_tracker_server/internal/config"
	mongoutils "goal_tracker_server/internal/mongo"
	"goal_tracker_server/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type dueTask struct {
	ID     primitive.ObjectID `bson:"_id"`
	UserID primitive.ObjectID `bson:"userId"`
	Name   string             `bson:"name"`
}

// StartDueTaskScheduler sends FCM notifications for tasks that are due soon.
// It is best-effort: if FCM isn't configured, it logs and no-ops.
func StartDueTaskScheduler(mongoClient *mongo.Client, cfg config.Config) {
	if cfg.FCMServiceAccountJSONPath == "" {
		log.Printf("[notifications] FCM_SERVICE_ACCOUNT_JSON_PATH not set; scheduler disabled")
		return
	}

	fcm, err := NewFCMClient(cfg.FCMServiceAccountJSONPath)
	if err != nil {
		log.Printf("[notifications] failed to init FCM client: %v (scheduler disabled)", err)
		return
	}

	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	db := mongoClient.Database(dbName)
	tasksColl := db.Collection("tasks")
	deviceTokensColl := db.Collection("deviceTokens")

	window := time.Duration(cfg.NotificationWindowMinutes) * time.Minute
	ticker := time.NewTicker(1 * time.Minute)

	go func() {
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now().UTC()
			dueEnd := now.Add(window)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			// de-dupe by notificationSentAt; notify once per due window tick.
			filter := bson.M{
				"status": string(models.TaskStatusNotStarted),
				"endAt": bson.M{
					"$gte": now,
					"$lte": dueEnd,
				},
				"$or": []bson.M{
					{"notificationSentAt": bson.M{"$exists": false}},
					{"notificationSentAt": bson.M{"$lt": now.Add(-1 * time.Minute)}},
				},
			}

			cur, err := tasksColl.Find(ctx, filter)
			if err != nil {
				cancel()
				continue
			}

			var due []dueTask
			for cur.Next(ctx) {
				var dt dueTask
				if err := cur.Decode(&dt); err == nil {
					due = append(due, dt)
				}
			}
			_ = cur.Close(ctx)

			for _, task := range due {
				// Fetch device tokens for that user.
				tokCur, err := deviceTokensColl.Find(ctx, bson.M{"userId": task.UserID})
				if err != nil {
					continue
				}

				var tokens []string
				for tokCur.Next(ctx) {
					var row struct {
						Token string `bson:"token"`
					}
					if err := tokCur.Decode(&row); err == nil && row.Token != "" {
						tokens = append(tokens, row.Token)
					}
				}
				_ = tokCur.Close(ctx)

				if len(tokens) == 0 {
					continue
				}

				data := map[string]string{
					"type":     "task_due",
					"taskId":   task.ID.Hex(),
					"taskName": task.Name,
					"dueAt":    dueEnd.Format(time.RFC3339),
				}

				if err := fcm.SendMulticast(ctx, tokens, data); err != nil {
					log.Printf("[notifications] send failed: %v", err)
					continue
				}

				// Mark tasks as notified to avoid spamming.
				_, _ = tasksColl.UpdateOne(ctx, bson.M{"_id": task.ID}, bson.M{"$set": bson.M{"notificationSentAt": now}})
			}

			cancel()
		}
	}()
}

