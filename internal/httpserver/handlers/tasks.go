package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"goal_tracker_server/internal/config"
	mongoutils "goal_tracker_server/internal/mongo"
	"goal_tracker_server/internal/httpserver/middleware"
	"goal_tracker_server/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TaskHandler struct {
	cfg     config.Config
	tasks   *mongo.Collection
	rewards *mongo.Collection
}

func NewTaskHandler(mongoClient *mongo.Client, cfg config.Config) *TaskHandler {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	db := mongoClient.Database(dbName)
	return &TaskHandler{
		cfg:     cfg,
		tasks:   db.Collection("tasks"),
		rewards: db.Collection("rewards"),
	}
}

type taskCreateRequest struct {
	Name        string     `json:"name"`
	Description string    `json:"description"`
	Status      *string   `json:"status"`
	EndAt       time.Time `json:"endAt"`
	Priority    string     `json:"priority"`
	Why         string     `json:"why"`
	RewardID    *string    `json:"rewardId"`
	Goal        *string    `json:"goal"`
}

type taskUpdateRequest struct {
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	Status      *string    `json:"status"`
	EndAt       *time.Time `json:"endAt"`
	Priority    *string    `json:"priority"`
	Why         *string    `json:"why"`
	RewardID    *string    `json:"rewardId"`
	Goal        *string    `json:"goal"`
}

type taskResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	EndAt       time.Time  `json:"endAt"`
	Priority    string     `json:"priority"`
	Why         string     `json:"why"`
	RewardID    *string    `json:"rewardId"`
	Goal        string     `json:"goal"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (h *TaskHandler) Create(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req taskCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Name == "" || req.Description == "" || req.Priority == "" || req.Why == "" || req.EndAt.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing mandatory fields"})
		return
	}

	status := string(models.TaskStatusNotStarted)
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}

	goal := string(models.TaskGoalDaily)
	if req.Goal != nil && *req.Goal != "" {
		goal = *req.Goal
	}

	var rewardOID *primitive.ObjectID
	if req.RewardID != nil && strings.TrimSpace(*req.RewardID) != "" {
		rid, err := primitive.ObjectIDFromHex(*req.RewardID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rewardId"})
			return
		}
		rewardOID = &rid
	}

	// Validate reward ownership if present.
	if rewardOID != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		count, err := h.rewards.CountDocuments(ctx, bson.M{"_id": *rewardOID, "userId": userID})
		if err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rewardId not found"})
			return
		}
	}

	doc := models.Task{
		ID:          primitive.NewObjectID(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Status:      models.TaskStatus(status),
		EndAt:       req.EndAt.UTC(),
		Priority:    req.Priority,
		Why:         req.Why,
		RewardID:    rewardOID,
		Goal:        models.TaskGoal(goal),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if _, err := h.tasks.InsertOne(ctx, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}

	c.JSON(http.StatusCreated, h.toResponse(doc))
}

func (h *TaskHandler) List(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	filter := bson.M{"userId": userID}

	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}
	if priority := c.Query("priority"); priority != "" {
		filter["priority"] = priority
	}
	if goal := c.Query("goal"); goal != "" {
		filter["goal"] = goal
	}

	// endAfter/endBefore accept RFC3339.
	if endAfter := c.Query("endAfter"); endAfter != "" {
		if t, err := time.Parse(time.RFC3339, endAfter); err == nil {
			filter["endAt"] = bson.M{"$gte": t.UTC()}
		}
	}
	if endBefore := c.Query("endBefore"); endBefore != "" {
		if t, err := time.Parse(time.RFC3339, endBefore); err == nil {
			if existing, ok := filter["endAt"].(bson.M); ok {
				existing["$lte"] = t.UTC()
				filter["endAt"] = existing
			} else {
				filter["endAt"] = bson.M{"$lte": t.UTC()}
			}
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	findOpts := bson.M{
		"sort": bson.M{"endAt": 1},
	}

	cur, err := h.tasks.Find(ctx, filter, findOptsToOptions(findOpts))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer cur.Close(ctx)

	var res []models.Task
	if err := cur.All(ctx, &res); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode failed"})
		return
	}

	out := make([]taskResponse, 0, len(res))
	for _, t := range res {
		out = append(out, h.toResponse(t))
	}
	c.JSON(http.StatusOK, out)
}

func (h *TaskHandler) Update(c *gin.Context) {
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

	var req taskUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var existing models.Task
	if err := h.tasks.FindOne(ctx, bson.M{"_id": id, "userId": userID}).Decode(&existing); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	// Final values (existing overridden by request pointers).
	status := string(existing.Status)
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}
	goal := string(existing.Goal)
	if req.Goal != nil && *req.Goal != "" {
		goal = *req.Goal
	}
	rewardOID := existing.RewardID
	if req.RewardID != nil {
		if strings.TrimSpace(*req.RewardID) == "" {
			rewardOID = nil
		} else {
			rid, err := primitive.ObjectIDFromHex(*req.RewardID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rewardId"})
				return
			}
			rewardOID = &rid
		}
	}

	// Validate reward ownership if present.
	if rewardOID != nil {
		count, err := h.rewards.CountDocuments(ctx, bson.M{"_id": *rewardOID, "userId": userID})
		if err != nil || count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rewardId not found"})
			return
		}
	}

	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.Status != nil && *req.Status != "" {
		update["status"] = status
	}
	if req.EndAt != nil && !req.EndAt.IsZero() {
		update["endAt"] = req.EndAt.UTC()
	}
	if req.Priority != nil {
		update["priority"] = *req.Priority
	}
	if req.Why != nil {
		update["why"] = *req.Why
	}
	if req.RewardID != nil {
		update["rewardId"] = rewardOID
	}
	if req.Goal != nil && *req.Goal != "" {
		update["goal"] = goal
	}
	update["updatedAt"] = time.Now().UTC()

	updateDoc := bson.M{"$set": update}
	if req.RewardID != nil {
		// rewardId handled in $set above.
	} else if req.Status != nil && *req.Status != "" && status == string(models.TaskStatusCompleted) {
		updateDoc["$unset"] = bson.M{"rewardId": ""}
	}

	res, err := h.tasks.UpdateOne(ctx, bson.M{"_id": id, "userId": userID}, updateDoc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	// Best-effort fetch updated doc.
	var updated models.Task
	_ = h.tasks.FindOne(ctx, bson.M{"_id": id, "userId": userID}).Decode(&updated)

	c.JSON(http.StatusOK, h.toResponse(updated))
}

func (h *TaskHandler) toResponse(t models.Task) taskResponse {
	var rewardID *string
	if t.RewardID != nil {
		s := t.RewardID.Hex()
		rewardID = &s
	}
	return taskResponse{
		ID:          t.ID.Hex(),
		Name:        t.Name,
		Description: t.Description,
		Status:      string(t.Status),
		EndAt:       t.EndAt,
		Priority:    t.Priority,
		Why:         t.Why,
		RewardID:    rewardID,
		Goal:        string(t.Goal),
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// findOptsToOptions converts a simple bson map to mongo FindOptions.
// This avoids importing options everywhere in handlers.
func findOptsToOptions(findOpts bson.M) *options.FindOptions {
	opts := options.Find()
	if sortVal, ok := findOpts["sort"]; ok {
		if sortDoc, ok := sortVal.(bson.M); ok {
			opts.SetSort(sortDoc)
		}
	}
	return opts
}

