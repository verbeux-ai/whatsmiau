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
- **Channel Management:** List, look up, create, follow, unfollow and mute WhatsApp channels (newsletters).
- **All Evolution API Message Types:** Compatible with all Evolution API message types for sending and receiving.
- **Message Reactions:** Support for sending and receiving emoji reactions.
- **Message Deletion:** Ability to delete messages for everyone.
- **Call Signaling:** Emits sanitized WhatsApp call signaling events for RTC integration work.

## Getting Started

To get a local copy up and running follow these simple steps.

### Prerequisites

- Go 1.26 or higher
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
| `S3_ENABLED` | Enable S3-compatible media storage. Cannot be combined with `GCS_ENABLED=true`. | `false` |
| `S3_BUCKET` | Existing bucket name; required when S3 is enabled. | `` |
| `S3_REGION` | Override the AWS SDK region. Otherwise use the standard AWS configuration, falling back to `us-east-1`. | `` |
| `S3_ENDPOINT` | Optional absolute HTTP(S) endpoint for an S3-compatible service. Leave empty for AWS S3. | `` |
| `S3_USE_PATH_STYLE` | Address objects as `endpoint/bucket/key`, useful for compatible services. | `false` |
| `S3_PREFIX` | Optional object prefix within the bucket, for example `core/media`. | `` |
| `S3_PUBLIC_URL` | Optional public/CDN base URL already pointing at the bucket, without query parameters. Otherwise return signed GET URLs. | `` |
| `S3_PRESIGN_TTL` | Signed URL lifetime, from `1s` to `168h`. Temporary credentials can expire sooner. | `1h` |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_SESSION_TOKEN` | Optional credentials for the standard AWS SDK credential chain. IAM roles and shared AWS profiles are also supported. | `` |
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

Media URL downloads accept only HTTP(S) destinations with public IP addresses.
Private, loopback, link-local (including cloud metadata), reserved and IPv6
transition addresses are blocked, including after DNS resolution and redirects.
Redirects between public destinations remain supported, with a limit of 10;
URLs containing credentials or IPv6 zone identifiers are rejected. Base64
`data:` media remains supported without a network request.

These downloads connect directly and do not use `HTTP_PROXY`, `HTTPS_PROXY` or
other environment proxies, which could bypass destination validation. The
WhatsApp instance proxy and `PROXY_NO_MEDIA` continue to control WhatsApp
connections and uploads/downloads performed by the WhatsApp client independently.
Use publicly reachable media URLs or Base64 instead of internal storage URLs.

### S3 media storage

S3 plugs into the same storage interface as GCS: received media includes
`mediaUrl` in the existing webhook payload, and profile pictures use the same
provider. The existing `saveMedia` setting continues to control message-media
uploads. Storage stays disabled unless a provider is explicitly enabled.

For a private AWS S3 bucket, configure the **core** environment:

```dotenv
GCS_ENABLED=false
S3_ENABLED=true
S3_BUCKET=your-existing-media-bucket
S3_REGION=us-east-1
S3_PREFIX=core/media
S3_PRESIGN_TTL=1h
```

Use an IAM role or provide the standard AWS credentials through your secret
manager. The core does not create buckets, change ACLs, set retention policies,
or copy existing GCS objects. Uploads use the AWS SDK transfer manager, including
multipart uploads for large inputs. Profile-picture uploads use an atomic
`If-None-Match: *` write: an existing object is reused across replicas/restarts
without an in-memory existence cache.

The role needs `s3:PutObject`, `s3:GetObject`, and `s3:AbortMultipartUpload` on
the configured object prefix. Bucket lifecycle/cleanup and any public/CDN access
are provisioned separately. No public-read ACL is sent. Providers must support
conditional writes to preserve `UploadBase64IfDontExists` semantics.

Without `S3_PUBLIC_URL`, returned URLs are temporary signed GET links. Do not
treat these links as permanent storage identifiers; their validity is limited
by `S3_PRESIGN_TTL` and the signing credentials. To return stable URLs, configure
`S3_PUBLIC_URL` with a public/CDN base URL that already includes the bucket path
when needed, such as `https://cdn.example.com/media-bucket`. The object prefix
and escaped key are appended to that URL; configuring it does not grant access.

For a compatible service, additionally set `S3_ENDPOINT` and, where required,
`S3_USE_PATH_STYLE=true`. Signed links point at that endpoint, so it must be
reachable by the consumers of the webhooks. Custom endpoints are trusted server
configuration; the media-download protections described above still apply to
URLs supplied through the API.

#### Test with Podman

The isolated test Compose file starts SeaweedFS with development-only credentials
on `127.0.0.1:19000`, independently of the application stack:

```sh
podman compose -p whatsmiau-s3-test -f docker-compose.s3-test.yml up -d
S3_TEST_ENDPOINT=http://127.0.0.1:19000 \
  AWS_ACCESS_KEY_ID=whatsmiau-s3-test \
  AWS_SECRET_ACCESS_KEY=whatsmiau-s3-test-secret \
  AWS_EC2_METADATA_DISABLED=true \
  go test -race ./lib/storage/...
podman compose -p whatsmiau-s3-test -f docker-compose.s3-test.yml down -v
```

The integration test creates a unique test bucket and removes it afterwards. It
verifies a multipart upload from a non-seekable reader, byte-for-byte download
through a signed URL, denial of unsigned access, and conditional reuse after
creating a fresh provider. Without `S3_TEST_ENDPOINT`, this test is explicitly
skipped; the HTTP-backed adapter and configuration tests still run. The PR
workflow runs the full core test suite and build with the disposable S3 service.

## API Documentation

WhatsMiau ships an English Swagger 2.0 contract for every application route:
canonical endpoints, Evolution API compatibility aliases, instance management,
messages, chats, groups, communities, channels, webhooks, calls, and documentation
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

## Channels (Newsletters)

WhatsApp channels are addressed by JIDs ending in `@newsletter`; the routes
also accept the bare numeric ID. The Evolution API has no channel routes, so
these are WhatsMiau-only endpoints under `/v1/instance/{instance}/newsletter`:

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/fetchAll` | Channels the number follows or administers. |
| `GET` | `/info?newsletterJid=` | One channel by JID. |
| `GET` | `/inviteInfo?inviteCode=` | One channel by invite code or `https://whatsapp.com/channel/...` link. |
| `POST` | `/create` | Create a channel: `name`, optional `description` and `picture` (URL, `data:` URI or base64). |
| `POST` | `/follow`, `/unfollow`, `/mute`, `/unmute` | Body `{"newsletterJid": "..."}`. |

Every lookup returns the same object: `id`, `name`, `description`,
`inviteCode`, `inviteUrl`, `pictureUrl`, `previewUrl`, `subscribersCount`,
`verified`, `state` (`active`, `suspended`, `geosuspended`), `role` (`owner`,
`admin`, `subscriber`, `guest`), `muted` and `canPublish` (true for owners and
admins). `role`, `muted` and `canPublish` are `null` when WhatsApp does not send
viewer data, as in invite lookups.

```bash
curl 'http://localhost:8080/v1/instance/my-instance/newsletter/fetchAll' \
-H 'apikey: YOUR_API_KEY'
```

Channel pictures sent by URL follow the media download rules above.

### Sending to a channel

The message routes accept `number` as `<id>@newsletter`. Text, image, video,
audio, sticker and poll messages are published to the channel; media is
uploaded unencrypted and marked as such, as the official clients do. Stickers
must already be WebP. Two channel-only options mirror the official app:
`correctAnswer` on `sendPoll` turns the poll into a quiz, and `question: true`
on `sendText` posts a question followers answer privately; both return `400`
for other chats. In channels, `quoted`, `mentioned` and `mentionsEveryOne` are
ignored, and link previews use only the small inline thumbnail. Document, list,
buttons, PIX, reaction, round video (PTV), location and contact messages return
`400`. The response includes
`key.serverId`, the server-assigned ID of the post, present only for channel
messages.

In `messages.upsert` webhooks, a quiz arrives as `pollCreationMessageV5`, with
`pollType: "QUIZ"` and `correctAnswer` in `message.pollCreationMessage`, and a
question arrives as `questionMessage`, with its text in `message.conversation`.

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
| `MESSAGES_EDITED` | `messages.edited` | Triggered when a message is edited; carries the original message key and the new content. Edited messages are delivered exclusively through this event. |
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


<!-- Security scan triggered at 2026-09-05 07:39:53 -->
