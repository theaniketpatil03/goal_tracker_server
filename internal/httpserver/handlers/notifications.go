package handlers

import (
	"context"
	"net/http"
	"time"

	"goal_tracker_server/internal/config"
	mongoutils "goal_tracker_server/internal/mongo"
	"goal_tracker_server/internal/httpserver/middleware"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type NotificationHandler struct {
	deviceTokens *mongo.Collection
}

func NewNotificationHandler(mongoClient *mongo.Client, cfg config.Config) *NotificationHandler {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	db := mongoClient.Database(dbName)
	return &NotificationHandler{
		deviceTokens: db.Collection("deviceTokens"),
	}
}

type registerFcmTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

func (h *NotificationHandler) RegisterFcmToken(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req registerFcmTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"userId": userID,
		"token":  req.Token,
	}

	update := bson.M{
		"$set": bson.M{
			"userId":    userID,
			"token":     req.Token,
			"updatedAt": time.Now().UTC(),
		},
		"$setOnInsert": bson.M{
			"createdAt": time.Now().UTC(),
		},
	}

	_, err := h.deviceTokens.UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token save failed"})
		return
	}

	c.Status(http.StatusNoContent)
}

// Optional helper to keep types stable if needed later.
func objectIDPtrFromHex(s string) *primitive.ObjectID {
	if s == "" {
		return nil
	}
	id, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		return nil
	}
	return &id
}

