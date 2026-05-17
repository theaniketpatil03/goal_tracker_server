package handlers

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
)

type MediaHandler struct {
	cfg    config.Config
	audios *mongo.Collection
	videos *mongo.Collection
}

func NewMediaHandler(mongoClient *mongo.Client, cfg config.Config) *MediaHandler {
	dbName := mongoutils.DatabaseName(cfg.MongoURI)
	if dbName == "" {
		dbName = "goal_tracker"
	}
	db := mongoClient.Database(dbName)

	return &MediaHandler{
		cfg:    cfg,
		audios: db.Collection("audios"),
		videos: db.Collection("videos"),
	}
}

func (h *MediaHandler) UploadAudio(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	coll := h.audios
	title, mimeType, fileName, err := h.saveMultipartToDisk(c, ".audio", "audio/mpeg")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc := models.Audio{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Title:     title,
		MimeType:  mimeType,
		FilePath:  fileName,
		CreatedAt: time.Now().UTC(),
	}

	if _, err := coll.InsertOne(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db insert failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": doc.ID.Hex(),
		"fileUrl": h.cfg.BaseURL + "/uploads/" + fileName,
	})
}

func (h *MediaHandler) UploadVideo(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	coll := h.videos
	title, mimeType, fileName, err := h.saveMultipartToDisk(c, ".video", "video/mp4")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	absVideoPath := filepath.Join(h.cfg.UploadDir, fileName)
	audioFileName := strings.TrimSuffix(fileName, filepath.Ext(fileName)) + ".mp3"
	absAudioPath := filepath.Join(h.cfg.UploadDir, audioFileName)

	doc := models.Video{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Title:     title,
		MimeType:  mimeType,
		FilePath:  fileName,
		AudioFilePath: "",
		CreatedAt: time.Now().UTC(),
	}

	// Best-effort audio extraction for Motivation background playback.
	// If ffmpeg isn't available or the file format is unsupported, we still upload the video.
	if err := h.extractAudioWithFFmpeg(absVideoPath, absAudioPath); err == nil {
		doc.AudioFilePath = audioFileName
	}

	if _, err := coll.InsertOne(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db insert failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": doc.ID.Hex(),
		"fileUrl": h.cfg.BaseURL + "/uploads/" + fileName,
	})
}

// saveMultipartToDisk stores the multipart field `file` into UPLOAD_DIR and returns metadata.
// It also uses the Mongo id hex in filename is not possible here without another lookup;
// for MVP we generate a filename based on time and original extension.
func (h *MediaHandler) saveMultipartToDisk(c *gin.Context, defaultTitleSuffix string, defaultMimeType string) (title, mimeType, fileName string, err error) {
	file, err := c.FormFile("file")
	if err != nil {
		return "", "", "", err
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".bin"
	}

	title = strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		base := strings.TrimSuffix(filepath.Base(file.Filename), ext)
		if base == "" {
			base = "file" + defaultTitleSuffix
		}
		title = base
	}

	mimeType = defaultMimeType
	if file.Header != nil && file.Header.Get("Content-Type") != "" {
		mimeType = file.Header.Get("Content-Type")
	}

	if err := os.MkdirAll(h.cfg.UploadDir, 0o755); err != nil {
		return "", "", "", err
	}

	base := strings.TrimSuffix(file.Filename, ext)
	base = strings.ReplaceAll(strings.ReplaceAll(base, " ", "_"), string(filepath.Separator), "_")
	fileName = time.Now().UTC().Format("20060102T150405.000Z") + "_" + base + ext
	fileName = strings.ReplaceAll(fileName, "..", "_")

	absPath := filepath.Join(h.cfg.UploadDir, fileName)

	dst, err := os.Create(absPath)
	if err != nil {
		return "", "", "", err
	}
	defer dst.Close()

	src, err := file.Open()
	if err != nil {
		return "", "", "", err
	}
	defer src.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", "", "", err
	}

	return title, mimeType, fileName, nil
}

func (h *MediaHandler) ListAudios(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := h.audios.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer cur.Close(ctx)

	type item struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		MimeType  string    `json:"mimeType"`
		FileURL   string    `json:"fileUrl"`
		CreatedAt time.Time `json:"createdAt"`
	}

	res := make([]item, 0)
	for cur.Next(ctx) {
		var a models.Audio
		if err := cur.Decode(&a); err != nil {
			continue
		}
		res = append(res, item{
			ID:        a.ID.Hex(),
			Title:     a.Title,
			MimeType:  a.MimeType,
			FileURL:   h.cfg.BaseURL + "/uploads/" + a.FilePath,
			CreatedAt: a.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, res)
}

func (h *MediaHandler) DeleteAudio(c *gin.Context) {
	h.deleteByID(c, h.audios)
}

func (h *MediaHandler) ListVideos(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := h.videos.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer cur.Close(ctx)

	type item struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		MimeType  string    `json:"mimeType"`
		FileURL   string    `json:"fileUrl"`
		AudioFileURL *string `json:"audioFileUrl"`
		CreatedAt time.Time `json:"createdAt"`
	}

	res := make([]item, 0)
	for cur.Next(ctx) {
		var v models.Video
		if err := cur.Decode(&v); err != nil {
			continue
		}
		res = append(res, item{
			ID:        v.ID.Hex(),
			Title:     v.Title,
			MimeType:  v.MimeType,
			FileURL:   h.cfg.BaseURL + "/uploads/" + v.FilePath,
			AudioFileURL: func() *string {
				if strings.TrimSpace(v.AudioFilePath) == "" {
					return nil
				}
				u := h.cfg.BaseURL + "/uploads/" + v.AudioFilePath
				return &u
			}(),
			CreatedAt: v.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, res)
}

func (h *MediaHandler) DeleteVideo(c *gin.Context) {
	h.deleteByID(c, h.videos)
}

func (h *MediaHandler) deleteByID(c *gin.Context, coll *mongo.Collection) {
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
	var doc bson.M
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if fp, ok := doc["filePath"].(string); ok && fp != "" {
		_ = os.Remove(filepath.Join(h.cfg.UploadDir, fp))
	}
	if afp, ok := doc["audioFilePath"].(string); ok && afp != "" {
		_ = os.Remove(filepath.Join(h.cfg.UploadDir, afp))
	}

	if _, err := coll.DeleteOne(ctx, filter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *MediaHandler) extractAudioWithFFmpeg(absVideoPath, absAudioPath string) error {
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", absVideoPath,
		"-vn",
		"-acodec", "libmp3lame",
		"-q:a", "2",
		absAudioPath,
	)
	// MVP: suppress verbose output.
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

