package vkgroup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.vk.com/method"
const apiVersion = "5.199"

type Client struct {
	token   string
	groupID int64
	http    *http.Client
}

func New(token string, groupID int64) *Client {
	return &Client{
		token:   token,
		groupID: groupID,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

type vkResponse struct {
	Response json.RawMessage `json:"response"`
	Error    *vkError        `json:"error"`
}

type vkError struct {
	Code    int    `json:"error_code"`
	Message string `json:"error_msg"`
}

func (c *Client) call(ctx context.Context, method string, params url.Values) (json.RawMessage, error) {
	params.Set("access_token", c.token)
	params.Set("v", apiVersion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		apiBase+"/"+method, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var vkResp vkResponse
	if err := json.Unmarshal(body, &vkResp); err != nil {
		return nil, err
	}
	if vkResp.Error != nil {
		return nil, fmt.Errorf("VK API error %d: %s", vkResp.Error.Code, vkResp.Error.Message)
	}
	return vkResp.Response, nil
}

func (c *Client) IsMember(ctx context.Context, userID int64) (bool, error) {
	params := url.Values{
		"group_id": {strconv.FormatInt(c.groupID, 10)},
		"user_id":  {strconv.FormatInt(userID, 10)},
	}
	raw, err := c.call(ctx, "groups.isMember", params)
	if err != nil {
		return false, err
	}
	var result int
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, err
	}
	return result == 1, nil
}

type SendMessageParams struct {
	PeerID     int64
	Text       string
	Attachment string
	Keyboard   string
	RandomID   int64
}

func (c *Client) SendMessage(ctx context.Context, p SendMessageParams) error {
	params := url.Values{
		"peer_id":   {strconv.FormatInt(p.PeerID, 10)},
		"message":   {p.Text},
		"random_id": {strconv.FormatInt(p.RandomID, 10)},
	}
	if p.Attachment != "" {
		params.Set("attachment", p.Attachment)
	}
	if p.Keyboard != "" {
		params.Set("keyboard", p.Keyboard)
	}
	_, err := c.call(ctx, "messages.send", params)
	return err
}

func (c *Client) SendEventAnswer(ctx context.Context, eventID string, userID, peerID int64, text string) error {
	eventData, _ := json.Marshal(map[string]string{"type": "show_snackbar", "text": text})
	params := url.Values{
		"event_id":   {eventID},
		"user_id":    {strconv.FormatInt(userID, 10)},
		"peer_id":    {strconv.FormatInt(peerID, 10)},
		"event_data": {string(eventData)},
	}
	_, err := c.call(ctx, "messages.sendEventAnswer", params)
	return err
}

// GetPhotoUploadServer возвращает URL для загрузки фото в сообщениях.
func (c *Client) GetPhotoUploadServer(ctx context.Context, peerID int64) (string, error) {
	params := url.Values{"peer_id": {strconv.FormatInt(peerID, 10)}}
	raw, err := c.call(ctx, "photos.getMessagesUploadServer", params)
	if err != nil {
		return "", err
	}
	var result struct {
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return result.UploadURL, nil
}

// SaveMessagesPhoto сохраняет загруженное фото и возвращает attachment-строку.
func (c *Client) SaveMessagesPhoto(ctx context.Context, server int, photo, hash string) (string, error) {
	params := url.Values{
		"server": {strconv.Itoa(server)},
		"photo":  {photo},
		"hash":   {hash},
	}
	raw, err := c.call(ctx, "photos.saveMessagesPhoto", params)
	if err != nil {
		return "", err
	}
	var photos []struct {
		ID      int64 `json:"id"`
		OwnerID int64 `json:"owner_id"`
	}
	if err := json.Unmarshal(raw, &photos); err != nil || len(photos) == 0 {
		return "", fmt.Errorf("не удалось сохранить фото")
	}
	return fmt.Sprintf("photo%d_%d", photos[0].OwnerID, photos[0].ID), nil
}
