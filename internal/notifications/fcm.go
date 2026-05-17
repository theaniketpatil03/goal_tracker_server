package notifications

import (
	"context"

	"firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type FCMClient struct {
	messaging *messaging.Client
}

func NewFCMClient(serviceAccountJSONPath string) (*FCMClient, error) {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(serviceAccountJSONPath))
	if err != nil {
		return nil, err
	}

	mc, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	return &FCMClient{messaging: mc}, nil
}

func (c *FCMClient) SendMulticast(ctx context.Context, tokens []string, data map[string]string) error {
	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Data:   data,
	}
	_, err := c.messaging.SendMulticast(ctx, msg)
	return err
}

