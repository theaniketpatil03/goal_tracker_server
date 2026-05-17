package handlers

import (
	"context"
	"net/http"
	"time"

	"goal_tracker_server/internal/config"
	mongoutils "goal_tracker_server/internal/mongo"
	"goal_tracker_server/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	cfg   config.Config
	users *mongo.Collection
}

func NewAuthHandler(mongoClient *mongo.Client, cfg config.Config) *AuthHandler {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	users := mongoClient.Database(dbName).Collection("users")

	// Best-effort index creation. If Mongo is unavailable, server will fail on connect anyway.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName("users_email_unique"),
	})

	return &AuthHandler{
		cfg:   cfg,
		users: users,
	}
}

type signupRequest struct {
	Email               string `json:"email" binding:"required"`
	Password            string `json:"password" binding:"required"`
	InvitationPassword string `json:"invitationPassword" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.InvitationPassword != h.cfg.AuthSignupPassword {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid invitation password"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password hashing failed"})
		return
	}

	user := models.User{
		ID:           primitive.NewObjectID(),
		Email:        req.Email,
		PasswordHash: string(hashed),
		CreatedAt:    time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = h.users.InsertOne(ctx, user)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	tokenStr, err := h.signToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"accessToken": tokenStr,
		"user": gin.H{
			"id":    user.ID.Hex(),
			"email": user.Email,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var user models.User
	if err := h.users.FindOne(ctx, bson.M{"email": req.Email}).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	tokenStr, err := h.signToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken": tokenStr,
		"user": gin.H{
			"id":    user.ID.Hex(),
			"email": user.Email,
		},
	})
}

func (h *AuthHandler) signToken(userID primitive.ObjectID) (string, error) {
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   userID.Hex(),
		Issuer:    h.cfg.JWTIssuer,
		Audience:  jwt.ClaimStrings{h.cfg.JWTAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(30 * 24 * time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

