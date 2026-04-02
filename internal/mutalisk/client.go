package mutalisk

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/openzerg/nydus/gen/go/mutaliskpb"
	"github.com/openzerg/nydus/gen/go/mutaliskpb/mutaliskpbconnect"
)

type Client struct {
	client mutaliskpbconnect.AgentClient
}

func NewClient(httpClient connect.HTTPClient, baseURL string) *Client {
	return &Client{
		client: mutaliskpbconnect.NewAgentClient(httpClient, baseURL),
	}
}

func (c *Client) GetOrCreateSessionByExternalId(ctx context.Context, externalId string, name string, purpose string, systemPrompt string, providerName string) (*mutaliskpb.SessionInfo, error) {
	req := &mutaliskpb.GetOrCreateSessionByExternalIdRequest{
		ExternalId: externalId,
		Name:       name,
	}
	if purpose != "" {
		req.Purpose = &purpose
	}
	if systemPrompt != "" {
		req.SystemPrompt = &systemPrompt
	}
	if providerName != "" {
		req.ProviderName = &providerName
	}

	resp, err := c.client.GetOrCreateSessionByExternalId(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("failed to get or create session: %w", err)
	}

	return resp.Msg, nil
}

func (c *Client) GetSessionByExternalId(ctx context.Context, externalId string) (*mutaliskpb.SessionInfo, error) {
	req := &mutaliskpb.GetSessionByExternalIdRequest{
		ExternalId: externalId,
	}

	resp, err := c.client.GetSessionByExternalId(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return resp.Msg, nil
}

func (c *Client) AddMessageToSession(ctx context.Context, sessionId string, role string, content string) (*mutaliskpb.MessageInfo, error) {
	req := &mutaliskpb.AddMessageToSessionRequest{
		SessionId: sessionId,
		Role:      role,
		Content:   content,
	}

	resp, err := c.client.AddMessageToSession(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	return resp.Msg, nil
}
