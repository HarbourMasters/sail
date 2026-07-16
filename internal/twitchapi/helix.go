package twitchapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const helixBase = "https://api.twitch.tv/helix"

// Helix is a minimal client for the few Twitch API calls this app needs.
type Helix struct {
	ClientID    string
	AccessToken string
}

// CustomReward is a broadcaster's channel point reward, trimmed to the
// fields this app uses.
type CustomReward struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	Cost                int    `json:"cost"`
	IsEnabled           bool   `json:"is_enabled"`
	IsUserInputRequired bool   `json:"is_user_input_required"`
}

// GetCustomRewards lists the broadcaster's channel point rewards, including
// ones created by other applications (e.g. the Twitch dashboard).
func (h *Helix) GetCustomRewards(ctx context.Context, broadcasterID string) ([]CustomReward, error) {
	var out struct {
		Data []CustomReward `json:"data"`
	}
	query := url.Values{"broadcaster_id": {broadcasterID}}
	if err := h.get(ctx, "/channel_points/custom_rewards", query, &out); err != nil {
		return nil, err
	}
	if out.Data == nil {
		// A nil slice marshals to JSON null, which the frontend .map()s without
		// a guard.
		out.Data = []CustomReward{}
	}
	return out.Data, nil
}

// SendChatMessageResult is the outcome of a send: whether the message was
// actually delivered, and — if not — why (e.g. held by AutoMod).
type SendChatMessageResult struct {
	Sent       bool
	DropReason string
}

// SendChatMessage posts a message to a channel's chat as the authenticated
// user. broadcasterID is the channel it's posted to and senderID is who it's
// posted as — for this app both are the logged-in user's own id. Requires the
// user:write:chat scope on the token.
func (h *Helix) SendChatMessage(ctx context.Context, broadcasterID, senderID, message string) (SendChatMessageResult, error) {
	body := map[string]string{
		"broadcaster_id": broadcasterID,
		"sender_id":      senderID,
		"message":        message,
	}
	var out struct {
		Data []struct {
			IsSent     bool `json:"is_sent"`
			DropReason struct {
				Message string `json:"message"`
			} `json:"drop_reason"`
		} `json:"data"`
	}
	if err := h.post(ctx, "/chat/messages", body, &out); err != nil {
		return SendChatMessageResult{}, err
	}
	if len(out.Data) == 0 {
		return SendChatMessageResult{}, errors.New("twitch returned no result for the sent message")
	}
	d := out.Data[0]
	return SendChatMessageResult{Sent: d.IsSent, DropReason: d.DropReason.Message}, nil
}

func (h *Helix) get(ctx context.Context, path string, query url.Values, out any) error {
	requestURL := helixBase + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	setAuthHeaders(req, h.ClientID, h.AccessToken)
	return doJSON(req, path, out)
}

func (h *Helix) post(ctx context.Context, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, helixBase+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthHeaders(req, h.ClientID, h.AccessToken)
	return doJSON(req, path, out)
}

// doJSON sends req and decodes a 200 body into out; a non-200 becomes an error
// carrying the body. label (the path) is for error context. eventsub's
// createSubscription keeps its own handling (202, no body).
func doJSON(req *http.Request, label string, out any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("call helix %s: %w", label, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("helix %s returned %d: %s", label, resp.StatusCode, body)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode helix %s response: %w", label, err)
	}
	return nil
}

// setAuthHeaders stamps the Twitch app credentials every helix and eventsub
// request needs.
func setAuthHeaders(req *http.Request, clientID, accessToken string) {
	req.Header.Set("Client-Id", clientID)
	req.Header.Set("Authorization", "Bearer "+accessToken)
}
