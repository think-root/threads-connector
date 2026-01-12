package threads

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	baseURL                = "https://graph.threads.net/v1.0"
	maxCharLimit           = 500
	containerReadyTimeout  = 30 * time.Second
	containerCheckInterval = 2 * time.Second
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
		HTTPClient:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) CreatePost(text string, imageURL string, externalURL string) (string, error) {
	chunks := splitText(text, maxCharLimit)

	// Note: externalURL will be posted as separate reply at the end

	if len(chunks) == 0 && imageURL == "" && externalURL == "" {
		return "", fmt.Errorf("no content to post")
	}

	var rootPostID string
	var previousPostID string

	for i, chunk := range chunks {
		// Use image only for the first chunk
		currentImageURL := ""
		if i == 0 {
			currentImageURL = imageURL
		}

		// If it's not the first post, it is a reply to the previous one
		replyToID := previousPostID

		creationID, err := c.createMediaContainer(chunk, currentImageURL, replyToID, "")
		if err != nil {
			return "", fmt.Errorf("failed to create media container for chunk %d: %w", i, err)
		}

		// Wait for container to be ready before publishing
		if err := c.waitForContainerReady(creationID); err != nil {
			return "", fmt.Errorf("container %d not ready: %w", i, err)
		}

		publishedID, err := c.publishMediaContainer(creationID)
		if err != nil {
			return "", fmt.Errorf("failed to publish chunk %d: %w", i, err)
		}

		if i == 0 {
			rootPostID = publishedID
		}
		previousPostID = publishedID

		// Delay between posts to ensure order and avoid rate limits
		time.Sleep(1 * time.Second)
	}

	// Handle case where text was empty but imageURL provided
	if len(chunks) == 0 && imageURL != "" {
		creationID, err := c.createMediaContainer("", imageURL, "", "")
		if err != nil {
			return "", fmt.Errorf("failed to create media container for image: %w", err)
		}
		if err := c.waitForContainerReady(creationID); err != nil {
			return "", fmt.Errorf("image container not ready: %w", err)
		}
		publishedID, err := c.publishMediaContainer(creationID)
		if err != nil {
			return "", fmt.Errorf("failed to publish image: %w", err)
		}
		rootPostID = publishedID
		previousPostID = publishedID
	}

	// 3. Post external URL as separate reply for user interaction
	if externalURL != "" && previousPostID != "" {
		// Wait longer to let the parent post propagate in Threads system
		log.Printf("Waiting 5 seconds before creating URL reply...")
		time.Sleep(5 * time.Second)

		replyToID := previousPostID
		creationID, err := c.createMediaContainer(externalURL, "", replyToID, "")
		if err != nil {
			return "", fmt.Errorf("failed to create media container for URL reply: %w", err)
		}

		if err := c.waitForContainerReady(creationID); err != nil {
			return "", fmt.Errorf("URL container not ready: %w", err)
		}

		publishedID, err := c.publishMediaContainer(creationID)
		if err != nil {
			return "", fmt.Errorf("failed to publish URL reply: %w", err)
		}

		log.Printf("URL reply published: %s", publishedID)
	} else if externalURL != "" {
		// No parent post, URL is the root post
		creationID, err := c.createMediaContainer(externalURL, "", "", "")
		if err != nil {
			return "", fmt.Errorf("failed to create media container for URL: %w", err)
		}

		if err := c.waitForContainerReady(creationID); err != nil {
			return "", fmt.Errorf("URL container not ready: %w", err)
		}

		publishedID, err := c.publishMediaContainer(creationID)
		if err != nil {
			return "", fmt.Errorf("failed to publish URL: %w", err)
		}
		rootPostID = publishedID
	}

	return rootPostID, nil
}

// waitForContainerReady polls the container status until it's FINISHED or times out
func (c *Client) waitForContainerReady(containerID string) error {
	endpoint := fmt.Sprintf("%s/%s?fields=status,error_message&access_token=%s",
		baseURL, containerID, url.QueryEscape(c.AccessToken))

	deadline := time.Now().Add(containerReadyTimeout)

	for time.Now().Before(deadline) {
		resp, err := c.HTTPClient.Get(endpoint)
		if err != nil {
			return fmt.Errorf("failed to check container status: %w", err)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read status response: %w", err)
		}

		var status struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			ErrorMessage string `json:"error_message"`
		}

		if err := json.Unmarshal(bodyBytes, &status); err != nil {
			return fmt.Errorf("failed to parse status response: %w", err)
		}

		log.Printf("[Threads API] Container %s status: %s", containerID, status.Status)

		switch status.Status {
		case "FINISHED":
			return nil
		case "PUBLISHED":
			return nil // Already published, that's fine
		case "ERROR":
			return fmt.Errorf("container processing failed: %s", status.ErrorMessage)
		case "EXPIRED":
			return fmt.Errorf("container expired before publishing")
		case "IN_PROGRESS":
			time.Sleep(containerCheckInterval)
		default:
			// Unknown status, wait a bit and retry
			time.Sleep(containerCheckInterval)
		}
	}

	return fmt.Errorf("timeout waiting for container to be ready")
}

func (c *Client) createMediaContainer(text, imageURL, replyToID, linkAttachment string) (string, error) {
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

	// Add link_attachment for URL preview card (only for TEXT posts)
	if linkAttachment != "" && mediaType == "TEXT" {
		params.Set("link_attachment", linkAttachment)
	}

	log.Printf("Creating media container. Type: %s, HasText: %v, HasImage: %v, HasLinkAttachment: %v",
		mediaType, text != "", imageURL != "", linkAttachment != "")

	resp, err := c.HTTPClient.PostForm(endpoint, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Log decoded response for readable Unicode
	c.logDecodedResponse("[Threads API] Create Container Response", resp.Status, bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return "", c.parseError(bodyBytes, resp.Status)
	}

	var result map[string]string
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}

	return result["id"], nil
}

func (c *Client) publishMediaContainer(creationID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/threads_publish", baseURL, c.UserID)

	params := url.Values{}
	params.Set("creation_id", creationID)
	params.Set("access_token", c.AccessToken)

	log.Printf("Publishing media container: %s", creationID)

	resp, err := c.HTTPClient.PostForm(endpoint, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Log decoded response for readable Unicode
	c.logDecodedResponse("[Threads API] Publish Response", resp.Status, bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return "", c.parseError(bodyBytes, resp.Status)
	}

	var result map[string]string
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}

	return result["id"], nil
}

// logDecodedResponse logs API response with decoded Unicode for readable non-ASCII characters
func (c *Client) logDecodedResponse(prefix, status string, body []byte) {
	// Try to parse and re-marshal with indentation for readable JSON
	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err == nil {
		// Re-marshal without HTML escaping to get readable Unicode
		encoder := json.NewEncoder(log.Writer())
		encoder.SetEscapeHTML(false)
		log.Printf("%s: Status=%s Body=", prefix, status)
		encoder.Encode(parsed)
	} else {
		// Fallback to raw string
		log.Printf("%s: Status=%s Body=%s", prefix, status, string(body))
	}
}

func (c *Client) parseError(body []byte, status string) error {
	var errResp APIErrorResponse

	if err := json.Unmarshal(body, &errResp); err != nil {
		// Fallback to raw body if parsing fails
		return fmt.Errorf("API error: %s - %s", status, string(body))
	}

	if errResp.Error.Message != "" {
		errMsg := fmt.Sprintf("API error: %s - %s", status, errResp.Error.Message)
		if errResp.Error.ErrorUserTitle != "" {
			errMsg += fmt.Sprintf(" (%s: %s)", errResp.Error.ErrorUserTitle, errResp.Error.ErrorUserMsg)
		}
		return fmt.Errorf("%s", errMsg)
	}

	return fmt.Errorf("API error: %s - %s", status, string(body))
}

type APIErrorResponse struct {
	Error struct {
		Message        string `json:"message"`
		Type           string `json:"type"`
		Code           int    `json:"code"`
		ErrorSubcode   int    `json:"error_subcode"`
		ErrorUserTitle string `json:"error_user_title"`
		ErrorUserMsg   string `json:"error_user_msg"`
		FBTraceID      string `json:"fbtrace_id"`
	} `json:"error"`
}

// TokenInfo contains information about the access token validity
type TokenInfo struct {
	IsValid           bool     `json:"is_valid"`
	ExpiresAt         int64    `json:"expires_at"`
	DataAccessExpires int64    `json:"data_access_expires_at"`
	Scopes            []string `json:"scopes"`
	UserID            string   `json:"user_id"`
	Application       string   `json:"application"`
}

type debugTokenResponse struct {
	Data TokenInfo `json:"data"`
}

// ValidateToken checks if the access token is valid by calling the debug_token endpoint
func (c *Client) ValidateToken() (*TokenInfo, error) {
	endpoint := fmt.Sprintf("%s/debug_token", baseURL)

	params := url.Values{}
	params.Set("access_token", c.AccessToken)
	params.Set("input_token", c.AccessToken)

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	resp, err := c.HTTPClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(bodyBytes, resp.Status)
	}

	var result debugTokenResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse token info: %w", err)
	}

	return &result.Data, nil
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
