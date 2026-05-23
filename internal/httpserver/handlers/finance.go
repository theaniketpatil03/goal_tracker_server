package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goal_tracker_server/internal/config"
	"goal_tracker_server/internal/httpserver/middleware"
	mongoutils "goal_tracker_server/internal/mongo"
	"goal_tracker_server/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FinanceHandler struct {
	cfg      config.Config
	finances *mongo.Collection
}

func NewFinanceHandler(mongoClient *mongo.Client, cfg config.Config) *FinanceHandler {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	return &FinanceHandler{
		cfg:      cfg,
		finances: mongoClient.Database(dbName).Collection("finances"),
	}
}

type financeCreateRequest struct {
	Category string    `json:"category"`
	Type     string    `json:"type"`
	Amount   float64   `json:"amount"`
	Time     time.Time `json:"time"`
	Comment  string    `json:"comment"`
}

type financeUpdateRequest struct {
	Category *string    `json:"category"`
	Type     *string    `json:"type"`
	Amount   *float64   `json:"amount"`
	Time     *time.Time `json:"time"`
	Comment  *string    `json:"comment"`
}

type financeResponse struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	Time      time.Time `json:"time"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func isValidFinanceType(t string) bool {
	return t == string(models.FinanceTypeIncome) || t == string(models.FinanceTypeExpense)
}

func (h *FinanceHandler) Create(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req financeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Category = strings.TrimSpace(req.Category)
	req.Type = strings.TrimSpace(req.Type)
	req.Comment = strings.TrimSpace(req.Comment)

	if req.Category == "" || !isValidFinanceType(req.Type) || req.Amount <= 0 || req.Time.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid fields"})
		return
	}

	now := time.Now().UTC()
	doc := models.FinanceEntry{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Category:  req.Category,
		Type:      models.FinanceType(req.Type),
		Amount:    req.Amount,
		Time:      req.Time.UTC(),
		Comment:   req.Comment,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if _, err := h.finances.InsertOne(ctx, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}

	c.JSON(http.StatusCreated, h.toResponse(doc))
}

func (h *FinanceHandler) List(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	filter := bson.M{"userId": userID}

	if entryType := strings.TrimSpace(c.Query("type")); entryType != "" {
		if !isValidFinanceType(entryType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid type filter"})
			return
		}
		filter["type"] = entryType
	}
	if category := strings.TrimSpace(c.Query("category")); category != "" {
		filter["category"] = category
	}

	if timeAfter := c.Query("timeAfter"); timeAfter != "" {
		if t, err := time.Parse(time.RFC3339, timeAfter); err == nil {
			if existing, ok := filter["time"].(bson.M); ok {
				existing["$gte"] = t.UTC()
				filter["time"] = existing
			} else {
				filter["time"] = bson.M{"$gte": t.UTC()}
			}
		}
	}
	if timeBefore := c.Query("timeBefore"); timeBefore != "" {
		if t, err := time.Parse(time.RFC3339, timeBefore); err == nil {
			if existing, ok := filter["time"].(bson.M); ok {
				existing["$lte"] = t.UTC()
				filter["time"] = existing
			} else {
				filter["time"] = bson.M{"$lte": t.UTC()}
			}
		}
	}

	if amountMin := c.Query("amountMin"); amountMin != "" {
		if min, err := strconv.ParseFloat(amountMin, 64); err == nil && min >= 0 {
			if existing, ok := filter["amount"].(bson.M); ok {
				existing["$gte"] = min
				filter["amount"] = existing
			} else {
				filter["amount"] = bson.M{"$gte": min}
			}
		}
	}
	if amountMax := c.Query("amountMax"); amountMax != "" {
		if max, err := strconv.ParseFloat(amountMax, 64); err == nil && max >= 0 {
			if existing, ok := filter["amount"].(bson.M); ok {
				existing["$lte"] = max
				filter["amount"] = existing
			} else {
				filter["amount"] = bson.M{"$lte": max}
			}
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	findOpts := findOptsToOptions(bson.M{"sort": bson.M{"time": -1}})
	cur, err := h.finances.Find(ctx, filter, findOpts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer cur.Close(ctx)

	res := make([]models.FinanceEntry, 0)
	if err := cur.All(ctx, &res); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode failed"})
		return
	}

	out := make([]financeResponse, 0, len(res))
	for _, entry := range res {
		out = append(out, h.toResponse(entry))
	}
	c.JSON(http.StatusOK, out)
}

func (h *FinanceHandler) Update(c *gin.Context) {
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

	var req financeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Type != nil {
		t := strings.TrimSpace(*req.Type)
		if t != "" && !isValidFinanceType(t) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid type"})
			return
		}
	}
	if req.Amount != nil && *req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be positive"})
		return
	}

	update := bson.M{}
	if req.Category != nil {
		cat := strings.TrimSpace(*req.Category)
		if cat == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category cannot be empty"})
			return
		}
		update["category"] = cat
	}
	if req.Type != nil {
		t := strings.TrimSpace(*req.Type)
		if t != "" {
			update["type"] = t
		}
	}
	if req.Amount != nil {
		update["amount"] = *req.Amount
	}
	if req.Time != nil && !req.Time.IsZero() {
		update["time"] = req.Time.UTC()
	}
	if req.Comment != nil {
		update["comment"] = strings.TrimSpace(*req.Comment)
	}
	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	update["updatedAt"] = time.Now().UTC()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	res, err := h.finances.UpdateOne(ctx, bson.M{"_id": id, "userId": userID}, bson.M{"$set": update})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	var updated models.FinanceEntry
	if err := h.finances.FindOne(ctx, bson.M{"_id": id, "userId": userID}).Decode(&updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
		return
	}

	c.JSON(http.StatusOK, h.toResponse(updated))
}

func (h *FinanceHandler) Delete(c *gin.Context) {
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

	res, err := h.finances.DeleteOne(ctx, bson.M{"_id": id, "userId": userID})
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

func (h *FinanceHandler) toResponse(entry models.FinanceEntry) financeResponse {
	return financeResponse{
		ID:        entry.ID.Hex(),
		Category:  entry.Category,
		Type:      string(entry.Type),
		Amount:    entry.Amount,
		Time:      entry.Time,
		Comment:   entry.Comment,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
	}
}
