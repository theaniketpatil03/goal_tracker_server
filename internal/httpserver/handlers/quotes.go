package handlers

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"time"

	"goal_tracker_server/internal/config"
	mongoutils "goal_tracker_server/internal/mongo"
	"goal_tracker_server/internal/models"
	"goal_tracker_server/internal/httpserver/middleware"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type QuoteHandler struct {
	cfg     config.Config
	quotes  *mongo.Collection
}

func NewQuoteHandler(mongoClient *mongo.Client, cfg config.Config) *QuoteHandler {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	return &QuoteHandler{
		cfg:    cfg,
		quotes: mongoClient.Database(dbName).Collection("quotes"),
	}
}

type quoteCreateRequest struct {
	Text   string `json:"text" binding:"required"`
	Author string `json:"author" binding:"required"`
}

type quoteUpdateRequest struct {
	Text   *string `json:"text"`
	Author *string `json:"author"`
}

func (h *QuoteHandler) Create(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req quoteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	doc := models.Quote{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Text:      req.Text,
		Author:    req.Author,
		CreatedAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if _, err := h.quotes.InsertOne(ctx, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":     doc.ID.Hex(),
		"text":   doc.Text,
		"author": doc.Author,
	})
}

func (h *QuoteHandler) List(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := h.quotes.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer cur.Close(ctx)

	res := make([]models.Quote, 0)
	if err := cur.All(ctx, &res); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode failed"})
		return
	}

	// Return only what the app needs (id/text/author/createdAt).
	c.JSON(http.StatusOK, res)
}

func (h *QuoteHandler) Update(c *gin.Context) {
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

	var req quoteUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	update := bson.M{}
	if req.Text != nil {
		update["text"] = *req.Text
	}
	if req.Author != nil {
		update["author"] = *req.Author
	}
	if len(update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "userId": userID}
	res, err := h.quotes.UpdateOne(ctx, filter, bson.M{"$set": update})
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

func (h *QuoteHandler) Delete(c *gin.Context) {
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
	res, err := h.quotes.DeleteOne(ctx, filter)
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

// TodayQuote returns a deterministic quote based on the user's device local date.
// Client should call: GET /api/quotes/today?localDate=YYYY-MM-DD
func (h *QuoteHandler) TodayQuote(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	localDate := c.Query("localDate")
	if localDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing localDate (YYYY-MM-DD)"})
		return
	}
	if _, err := time.Parse("2006-01-02", localDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "localDate must be YYYY-MM-DD"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := h.quotes.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer cur.Close(ctx)

	var all []models.Quote
	if err := cur.All(ctx, &all); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode failed"})
		return
	}
	if len(all) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no quotes"})
		return
	}

	seedKey := fmt.Sprintf("%s:%s", userID.Hex(), localDate)
	hsh := fnv.New64a()
	_, _ = hsh.Write([]byte(seedKey))
	idx := int(hsh.Sum64() % uint64(len(all)))

	q := all[idx]
	c.JSON(http.StatusOK, gin.H{
		"id":     q.ID.Hex(),
		"text":   q.Text,
		"author": q.Author,
	})
}

