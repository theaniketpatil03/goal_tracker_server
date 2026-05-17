package middleware

import (
	"net/http"
	"strings"
	"time"

	"goal_tracker_server/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func JWTAuth(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || token == nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Basic validations (jwt lib validates exp/iat when calling claims.Valid(); we do explicit checks here).
		if claims.Subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing subject"})
			return
		}

		if claims.Issuer != "" && claims.Issuer != cfg.JWTIssuer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid issuer"})
			return
		}

		if len(claims.Audience) > 0 {
			found := false
			for _, aud := range claims.Audience {
				if aud == cfg.JWTAudience {
					found = true
					break
				}
			}
			if !found {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid audience"})
				return
			}
		}

		// jwt/v5 NumericDate doesn't expose VerifyExpiresAt; compare times directly.
		if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now()) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
			return
		}

		userID, err := primitive.ObjectIDFromHex(claims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (primitive.ObjectID, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return primitive.NilObjectID, false
	}
	id, ok := v.(primitive.ObjectID)
	return id, ok
}

