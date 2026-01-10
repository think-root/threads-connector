package threads

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	baseURL      = "https://graph.threads.net/v1.0"
	maxCharLimit = 500
)

type Client struct {
	UserID      string
	AccessToken string
	HTTPClient  *http.Client
}

func NewClient(userID, accessToken string) *Client {
	return &Client{
		UserID:      userID,
		AccessToken: accessToken,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

type creationResponse struct {
	ID string `json:"id"`
}

type publishResponse struct {
	ID string `json:"id"`
}

// CreatePost creates a post, handling text splitting, images, and URL appending.
func (c *Client) CreatePost(text string, imageURL string, externalURL string) (string, error) {
	// 1. Split text into chunks
	chunks := splitText(text, maxCharLimit)

	// 2. If allow external URL, append valid URL as the last chunk or to the last chunk if it fits
	if externalURL != "" {
		lastChunkIdx := len(chunks) - 1
		if len(chunks) > 0 {
			// Try to append to last chunk
			if len(chunks[lastChunkIdx])+len("\n\n")+len(externalURL) <= maxCharLimit {
				chunks[lastChunkIdx] = chunks[lastChunkIdx] + "\n\n" + externalURL
			} else {
				// Create new chunk
				chunks = append(chunks, externalURL)
			}
		} else {
			chunks = append(chunks, externalURL)
		}
	}

	if len(chunks) == 0 && imageURL == "" {
		return "", fmt.Errorf("no content to post")
	}

	var rootPostID string
	var previousPostID string

	// 3. Iterate and post
	for i, chunk := range chunks {
		// Use image only for the first chunk
		currentImageURL := ""
		if i == 0 {
			currentImageURL = imageURL
		}

		// Create media container
		// If it's not the first post, it is a reply to the previous one
		replyToID := previousPostID

		// If this is the *first* text chunk but we have no text (just image?), handle that.
		// But splitText returns at least one chunk if text is not empty.
		// If text is empty but image exists, splitText returns empty slice? No, create logic.

		creationID, err := c.createMediaContainer(chunk, currentImageURL, replyToID)
		if err != nil {
			return "", fmt.Errorf("failed to create media container for chunk %d: %w", i, err)
		}

		// Wait/Sleep? Threads might need a moment?
		// Usually publish is immediate for text.

		// Publish
		publishedID, err := c.publishMediaContainer(creationID)
		if err != nil {
			return "", fmt.Errorf("failed to publish chunk %d: %w", i, err)
		}

		if i == 0 {
			rootPostID = publishedID
		}
		previousPostID = publishedID

		// Simple delay to ensure order and avoid rate limits
		time.Sleep(1 * time.Second)
	}

	// Handle case where text was empty but imageURL provided
	if len(chunks) == 0 && imageURL != "" {
		creationID, err := c.createMediaContainer("", imageURL, "")
		if err != nil {
			return "", fmt.Errorf("failed to create media container for image: %w", err)
		}
		publishedID, err := c.publishMediaContainer(creationID)
		if err != nil {
			return "", fmt.Errorf("failed to publish image: %w", err)
		}
		return publishedID, nil
	}

	return rootPostID, nil
}

func (c *Client) createMediaContainer(text, imageURL, replyToID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/threads", baseURL, c.UserID)

	params := url.Values{}
	params.Set("access_token", c.AccessToken)

	mediaType := "TEXT"
	if imageURL != "" {
		mediaType = "IMAGE"
		params.Set("image_url", imageURL)
	}
	params.Set("media_type", mediaType)

	if text != "" {
		params.Set("text", text)
	}

	if replyToID != "" {
		params.Set("reply_to_id", replyToID)
	}

	resp, err := c.HTTPClient.PostForm(endpoint, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (create): %s - %s", resp.Status, string(body))
	}

	var result creationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

func (c *Client) publishMediaContainer(creationID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/threads_publish", baseURL, c.UserID)

	params := url.Values{}
	params.Set("creation_id", creationID)
	params.Set("access_token", c.AccessToken)

	// Publishing can sometimes fail if container is not ready.
	// Simple retry loop could be added here, but for now we trust standard flow.

	resp, err := c.HTTPClient.PostForm(endpoint, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (publish): %s - %s", resp.Status, string(body))
	}

	var result publishResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

// splitText splits a string into chunks of max length, respecting word boundaries.
func splitText(text string, limit int) []string {
	if text == "" {
		return []string{}
	}
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	words := strings.Fields(text)
	currentChunk := ""

	for _, word := range words {
		// +1 for space
		if len(currentChunk)+len(word)+1 > limit {
			chunks = append(chunks, currentChunk)
			currentChunk = word
		} else {
			if currentChunk != "" {
				currentChunk += " "
			}
			currentChunk += word
		}
	}
	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}
	return chunks
}
