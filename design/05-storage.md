# 05 — File Storage

> **External dependency**: RustFS is deployed in a separate Docker Compose stack and is not managed by this project. The backend connects to it over the shared `pangolin` Docker network.

---

## RustFS Setup

Before starting this stack, deploy RustFS in its own Compose file attached to the `pangolin` external network:

```yaml
# rustfs/docker-compose.yml (separate stack — not in this repo)
services:
  rustfs:
    image: rustfs/rustfs:latest
    restart: unless-stopped
    environment:
      RUSTFS_ACCESS_KEY: ${RUSTFS_ACCESS_KEY}
      RUSTFS_SECRET_KEY: ${RUSTFS_SECRET_KEY}
    volumes:
      - rustfs_data:/data
    healthcheck:
      test: ['CMD-SHELL', 'wget -qO- http://localhost:9000/health || exit 1']
      interval: 5s
      retries: 5
    expose:
      - 9000
    networks:
      - pangolin

volumes:
  rustfs_data:
networks:
  pangolin:
    external: true
```

Before starting this stack:

1. Create the `fabyoumeme-assets` bucket
2. Create credentials (`RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`) and note them for this project's `.env`

Once RustFS is running on `pangolin`, the backend resolves it by the container name `rustfs`.

---

## Storage Interface

The storage layer is wrapped behind a `Storage` interface in `internal/storage/`. The concrete implementation uses `aws-sdk-go-v2/s3` pointed at RustFS. Swapping to MinIO or any other S3-compatible store requires changing only the concrete implementation — call sites are unaffected.

---

## Access Model

- All game assets stored in a single **private** bucket (`fabyoumeme-assets`)
- **No public bucket access** — every read goes through a short-lived pre-signed URL
- Pre-signed download URLs: **15-minute TTL**
- Pre-signed upload URLs: issued only to authenticated admin sessions

---

## Object Key Convention

```plain
packs/{pack_id}/items/{item_id}/{original_filename}
```

---

## Upload Flow

Admin uploads a new image for an item:

```plain
1. POST /api/packs/:id/items
     Body: { payload, payload_version }    ← no image yet
     Response: { item_id, ... }

2. POST /api/assets/upload-url
     Body: { pack_id, item_id, filename, mime_type, size_bytes }
     Server validates:
       - mime_type ∈ { image/jpeg, image/png, image/webp }
       - magic bytes match the declared mime_type (Go image.DecodeConfig)
       - size_bytes ≤ MAX_UPLOAD_SIZE_BYTES (default 2 MB)
     Response: { upload_url, media_key }

3. Frontend PUT {file} to upload_url directly (client → RustFS, bypasses backend)

4. PATCH /api/packs/:id/items/:item_id
     Body: { media_key }
     Server stores media_key on the item record — upload confirmed
```

The item must exist before the upload URL is requested (step 1 before step 2). The backend has no S3 webhook; step 4 is the explicit frontend confirmation.

**MIME validation detail**: the server first checks the `mime_type` field against the allowed list, then reads the first ~512 bytes of the file using `image.DecodeConfig` to validate the magic bytes. Checking the `Content-Type` request header alone is insufficient — an attacker can send any header value with any file content. Magic byte inspection cannot be bypassed via metadata manipulation.

**Frontend preview**: before submitting the upload, the frontend should render the image with a `<img src={URL.createObjectURL(file)}>` element so the admin can verify the correct file was selected.

---

## Download Flow — Embedded in WebSocket Events

Clients **do not** request download URLs individually. When the backend broadcasts `round_started`, it generates pre-signed GET URLs (15-minute TTL) for all assets in the round and embeds them directly in the event payload. The client receives everything needed to render the round in a single server push — no extra round-trips.

`POST /api/assets/download-url` is retained only for **admin preview** (viewing items outside a live game).

### Pre-signed URL Policy

Pre-signed URLs are generated with `response-content-disposition=attachment`. This forces the browser to treat the resource as a file download rather than rendering it inline, which reduces the casual-sharing risk (a forwarded URL opens a download dialog instead of displaying the image directly in the browser).

For in-game rendering, the frontend loads the asset into an `<img>` element via an object URL created from a `fetch()` response — the image is never directly navigable.

---

## Asset Lifecycle

When a game pack is soft-deleted (`deleted_at` set), its items remain in the DB for historical game data integrity. The corresponding RustFS objects are **not** deleted automatically — they remain in the bucket. If storage reclamation is needed, an admin can manually purge objects from RustFS after verifying no `finished` rooms reference them.

There is no automated garbage collection of orphaned objects; this is an acceptable trade-off at self-hosted scale.
