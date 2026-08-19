# WhatsMiau

![logo-whatsmiau](logo.png)

WhatsMiau is a backend service for WhatsApp, built with Go. It uses the Whatsmeow library to connect to WhatsApp and provides an HTTP API to send and receive messages.

[Community Whatsapp (BR)](https://chat.whatsapp.com/FXMrTY552nOBFXU71Be8Zh)
## About The Project

This project provides a robust, scalable, and production-ready solution for integrating WhatsApp functionalities into your applications. It is extremely lightweight, consuming very little memory, making it ideal for resource-constrained environments.

It's designed to be compatible with the Evolution API, making it a flexible choice for developers familiar with that ecosystem.

## Features

- **Lightweight & Efficient:** Optimized for low memory consumption.
- **Production Ready:** Stable and reliable for use in production environments.
- **WhatsApp Integration:** Connects to WhatsApp to send and receive messages.
- **HTTP API:** Exposes an HTTP API for easy integration with other services.
- **Redis Support:** Uses Redis for session storage and caching.
- **SQLite Database:** Utilizes SQLite for persistent data storage.
- **Environment-based Configuration:** Easily configure the application using environment variables.
- **Structured Logging:** Implements structured logging with Zap for better monitoring and debugging.
- **Group & Community Management:** Full support for WhatsApp group and community operations.
- **All Evolution API Message Types:** Compatible with all Evolution API message types for sending and receiving.
- **Message Reactions:** Support for sending and receiving emoji reactions.
- **Message Deletion:** Ability to delete messages for everyone.
- **Call Signaling:** Emits sanitized WhatsApp call signaling events for RTC integration work.

## Getting Started

To get a local copy up and running follow these simple steps.

### Prerequisites

- Go 1.24 or higher
- Redis
- SQLite

### Installation

1. Clone the repo
   ```sh
   git clone https://github.com/verbeux-ai/whatsmiau.git
   ```
2. Install Go packages
   ```sh
   go mod tidy
   ```
3. Set up your environment variables by copying `.env.example` to `.env` and filling in the required values.
   ```sh
   cp .env.example .env
   ```
4. Run the application
   ```sh
   go run main.go
   ```

## Running with Docker

You can also run the application using Docker and Docker Compose.

1.  **Build and run the containers:**
    ```sh
    docker-compose up -d --build
    ```
2.  **View the logs:**
    ```sh
    docker-compose logs -f
    ```
3.  **Stop the containers:**
    ```sh
    docker-compose down
    ```

## Docker Image

Official Docker images are available on Docker Hub.

- **Latest stable release:** `impedr029/whatsmiau:vX.Y.Z` [(see versions)](https://github.com/verbeux-ai/whatsmiau/tags)
- **Development version:** `impedr029/whatsmiau:develop`

You can pull the latest stable image with (example):
```sh
docker pull impedr029/whatsmiau:vX.Y.Z
```

Or the development image with:
```sh
docker pull impedr029/whatsmiau:develop
```

## Configuration

The application is configured using environment variables. The following variables are available:

| Variable | Description | Default |
| --- | --- | --- |
| `PORT` | The port the server will run on. | `8080` |
| `DEBUG_MODE` | Enable or disable debug mode. | `false` |
| `DEBUG_WHATSMEOW` | Enable or disable debug mode for Whatsmeow. | `false` |
| `REDIS_URL` | The URL of the Redis server. | `localhost:6379` |
| `REDIS_PASSWORD` | The password for the Redis server. | `` |
| `REDIS_TLS` | Enable or disable TLS for Redis. | `false` |
| `API_KEY` | The API key to protect the service. | `` |
| `DIALECT_DB` | The database dialect to use (`sqlite3` or `postgres`). | `sqlite3` |
| `DB_URL` | The database connection URL. | `file:data.db?_foreign_keys=on` |
| `GCS_ENABLED` | Enable or disable Google Cloud Storage. | `false` |
| `GCS_BUCKET` | The GCS bucket name. | `whatsmiau` |
| `GCS_URL` | The GCS URL. | `https://storage.googleapis.com` |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to GCP service account JSON key. | `` |
| `GCL_APP_NAME` | The GCL application name. | `whatsmiau-br-1` |
| `GCL_ENABLED` | Enable or disable Google Cloud Logging. | `false` |
| `GCL_PROJECT_ID` | The GCL project ID. | `` |
| `EMITTER_BUFFER_SIZE` | The emitter buffer size. | `2048` |
| `EMITTER_WORKERS` | The number of emitter workers. | `50` |
| `HANDLER_SEMAPHORE_SIZE` | The handler semaphore size. | `512` |
| `PROXY_ADDRESSES` | A comma-separated list of proxy addresses. Example: `SOCKS5://user:pass@host:port,HTTP://host:port` | `` |
| `PROXY_STRATEGY` | The strategy to use when selecting a proxy from the list (`RANDOM`). | `RANDOM` |
| `PROXY_NO_MEDIA` | If set to `true`, media will not be sent through the proxy. | `false` |
| `CALLS_ENABLED` | Enables the experimental authenticated 1:1 call API. | `false` |
| `CALL_DEBUG` | Logs sanitized incoming offer shapes for controlled protocol comparisons; no keys, ciphertexts, IDs, or JIDs. | `false` |

## API Documentation

WhatsMiau ships an English Swagger 2.0 contract for every application route:
canonical endpoints, Evolution API compatibility aliases, instance management,
messages, chats, groups, communities, webhooks, calls, and documentation
delivery itself.

- Open the Swagger UI at `GET /docs`. Use Swagger's **Authorize** dialog to
  enter the regular `apikey` before sending “Try it out” API requests.
- Download the machine-readable contract with `GET /v1/swagger.json` or
  `GET /v1/swagger.yaml`, using the standard `apikey` header.
- All `/v1` operations remain authenticated exclusively through `apikey`. A
  Swagger UI page does not change that requirement; it only adds the header
  when an operation is executed.
- The Swagger UI and its read-only contract documents are public so the UI can
  load before the key is entered. API operations and the `/v1` contract
  downloads remain protected by the normal `apikey` middleware.

Use HTTPS whenever the UI is exposed outside a trusted development network,
because the API key is sent in request headers during “Try it out”.

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/verbeux-ai/whatsmiau/tags).

## Compatibility

This API is designed to be compatible with the Evolution API. This means that you can use clients and tools designed for the Evolution API with this project.

It exclusively supports webhooks in the Evolution API format, offering two distinct approaches for their implementation, providing flexibility for different use cases.

## Migration from Evolution API

WhatsMiau is designed to be a lightweight, drop-in replacement for the Evolution API. If you are running WhatsMiau on the same host and port as your previous Evolution API instance, migration is seamless.

Since WhatsMiau maintains compatibility with the Evolution API's routes, you only need to stop your Evolution API server and start the WhatsMiau server. No changes to your existing API calls are necessary.

### Example

For instance, if you were sending a text message using a `curl` command to an Evolution API server running on `localhost:8080`, the exact same command will work with WhatsMiau.

**Before (Evolution API):**
```bash
curl -X POST 'http://localhost:8080/message/sendText/my-instance' \
-H 'Content-Type: application/json' \
-H 'apikey: YOUR_API_KEY' \
-d ".{\"number\": \"1234567890\",\"textMessage\": {\"text\": \"Hello from Evolution API!\"}}"
```

**After (WhatsMiau):**

Simply point your application to the WhatsMiau server URL. The same request will be handled by WhatsMiau:
```bash
curl -X POST 'http://localhost:8080/v1/message/sendText/my-instance' \
-H 'Content-Type: application/json' \
-H 'apikey: YOUR_API_KEY' \
-d ".{\"number\": \"1234567890\",\"textMessage\": {\"text\": \"Hello from WhatsMiau!\"}}"
```

## Supported Events

Webhook configuration and webhook payloads use different event identifiers to preserve Evolution API compatibility. Configure subscriptions with the uppercase value. The emitted payload uses the lowercase value in its `event` field.

| Configuration value | Payload `event` value | Description |
|---------------------|-----------------------|-------------|
| `MESSAGES_UPSERT` | `messages.upsert` | Triggered when a new message is received. |
| `MESSAGES_UPDATE` | `messages.update` | Triggered when a message status changes, such as a read receipt. |
| `MESSAGES_DELETE` | `messages.delete` | Triggered when a message is deleted for everyone. |
| `MESSAGES_SET` | `messages.set` | Triggered during full-history synchronization when `syncFullHistory` is enabled. |
| `CONTACTS_UPSERT` | `contacts.upsert` | Triggered when a contact is created or updated. |
| `GROUP_PARTICIPANTS_UPDATE` | `group-participants.update` | Triggered when participants are added, removed, promoted, or demoted. |
| `CONNECTION_UPDATE` | `connection.update` | Triggered when connection state changes. |
| `CALL` | `call` | Triggered for call signaling; no media data, SDP, or call/session keys are included. |

Payload-style values such as `messages.upsert` remain accepted in configuration for compatibility with older Manager UI versions. WhatsMiau stores and returns configured values as provided; it normalizes them only when matching subscriptions.

## Contributors

<a href="https://github.com/verbeux-ai/whatsmiau/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=verbeux-ai/whatsmiau" />
</a>

## Did you like project?
Donate: https://buy.stripe.com/8x28wI5vKfPbe9b8ih1VK0f
