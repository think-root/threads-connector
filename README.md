# threads-connector

This project is part of the [content-maestro](https://github.com/think-root/content-maestro) repository (conceptually). If you want Threads integration and automatic publishing of posts there as well, you need to deploy this app.

## Description

A Go-based HTTP API server that integrates with the Threads Graph API. It exposes REST endpoints for creating posts. It handles:

- **Two-step posting process**: Creating a media container and publishing it.
- **Auto-Threading**: Automatically splits long text (>500 chars) into multiple threaded posts.
- **Image Support**: Attaching an image to the first post (requires a public URL).
- **URL Handling**: Posts external URL as a separate reply for better user interaction (with link preview card).

## Prerequisites

Before running the service, make sure you have:

- [Go](https://go.dev/dl/) 1.21+
- A Threads Account
- A Threads User ID
- A valid [Threads Access Token](https://developers.facebook.com/docs/threads/overview) (User Token) with the following scopes:
  - `threads_basic` — Required for all API calls
  - `threads_content_publish` — Required for publishing posts
  - `threads_manage_replies` — Required for posting URL as a separate reply

## Setup

1. **Clone this repository.**
2. **Install dependencies:**

   ```bash
   go mod tidy
   ```

3. **Create a `.env` file:**

   ```bash
   cp .env.example .env
   ```

   Then populate the required variables:

   ```bash
   THREADS_USER_ID=your_threads_user_id
   THREADS_ACCESS_TOKEN=your_valid_access_token
   PORT=8080
   API_KEY=your_secret_api_key
   ```

4. **Run the server:**

   ```bash
   go run cmd/server/main.go
   ```

   Optional production build:

   ```bash
   go build -o threads-connector cmd/server/main.go
   ./threads-connector
   ```

   The server listens on `http://localhost:8080` unless `PORT` overrides it.

## API

### POST `/threads/post`

Creates and publishes a Threads post (or thread if text is long).

**Security:**
Requires `X-API-Key` header with the value matching your `API_KEY` environment variable.

#### Request

**Content-Type:** `application/json`

| Parameter   | Type   | Required | Description                                                                 |
| ----------- | ------ | -------- | --------------------------------------------------------------------------- |
| `text`      | string | No*      | Main post content. *Required if no image. Splits >500 chars.                |
| `image_url` | string | No*      | Public URL of an image to attach (first post only). *Required if no text.   |
| `url`       | string | No       | External link posted as a separate reply                                    |

#### Examples

**Simple post:**

```bash
curl -X POST "http://localhost:8080/threads/post" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_secret_api_key" \
  -d '{"text": "Hello, Threads!"}'
```

**Post with Image and Link:**

```bash
curl -X POST "http://localhost:8080/threads/post" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_secret_api_key" \
  -d '{
    "text": "Check out this amazing view!",
    "image_url": "https://example.com/image.jpg",
    "url": "https://example.com"
  }'
```

#### Response (200 OK)

```json
{
  "post_id": "1234567890"
}
```

**Error:**

```json
{
  "error": "Failed to create post: ..."
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
