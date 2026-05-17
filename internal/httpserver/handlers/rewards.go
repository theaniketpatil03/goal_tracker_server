package handlers

import (
	"context"
	"net/http"
	"time"

	"goal_tracker_server/internal/config"
	mongoutils "goal_tracker_server/internal/mongo"
	"goal_tracker_server/internal/httpserver/middleware"
	"goal_tracker_server/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RewardHandler struct {
	cfg     config.Config
	rewards *mongo.Collection
}

func NewRewardHandler(mongoClient *mongo.Client, cfg config.Config) *RewardHandler {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	return &RewardHandler{
		cfg:     cfg,
		rewards: mongoClient.Database(dbName).Collection("rewards"),
	}
}

type rewardCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type rewardUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *RewardHandler) Create(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req rewardCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	doc := models.Reward{
		ID:          primitive.NewObjectID(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if _, err := h.rewards.InsertOne(ctx, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          doc.ID.Hex(),
		"name":        doc.Name,
		"description": doc.Description,
	})
}

func (h *RewardHandler) List(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := h.rewards.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer cur.Close(ctx)

	// Non-nil slice so JSON is always [] for empty results (nil slice encodes as null).
	res := make([]models.Reward, 0)
	if err := cur.All(ctx, &res); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode failed"})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *RewardHandler) Update(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req rewardUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "userId": userID}
	res, err := h.rewards.UpdateOne(ctx, filter, bson.M{"$set": update})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *RewardHandler) Delete(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "userId": userID}
	res, err := h.rewards.DeleteOne(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	if res.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

