package main

import (
	"log"

	"goal_tracker_server/internal/config"
	"goal_tracker_server/internal/httpserver"
	"goal_tracker_server/internal/notifications"
	"goal_tracker_server/internal/models"
	"goal_tracker_server/internal/mongo"
)

func main() {
	cfg := config.MustLoad()

	client, err := mongo.Connect(cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	defer func() { _ = client.Disconnect(nil) }()

	if err := models.EnsureIndexes(client, cfg); err != nil {
		log.Printf("ensure indexes: %v", err)
	}

	notifications.StartDueTaskScheduler(client, cfg)

	router := httpserver.NewRouter(cfg, client)

	addr := ":" + cfg.Port
	log.Printf("listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server run: %v", err)
	}
}

