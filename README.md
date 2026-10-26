# Open Streaming

Local media server that finds a movie, pulls it over BitTorrent, and plays it on devices already on the LAN.

Search a title, add the info-hash, and the torrent engine starts the swarm while HTTP range requests serve the video file. A DLNA **Digital Media Server (DMS)** publishes those torrents so a TV, Kodi, or VLC on the same network can browse and play them. The end goal is full DLNA on the LAN: keep the DMS, and add a **Digital Media Renderer (DMR)** path so playback can be pushed to a renderer instead of only pulled from the server.

## What it is for

Open Streaming is a single Go process meant to run on a machine in the house (a PC, a mini PC, a NAS). It is not a public streaming site and it does not host a catalog of files. It:

1. Resolves a title to an IMDb id, then to torrent streams.
2. Joins the swarm for the chosen info-hash and prioritizes the largest video file.
3. Serves that file over HTTP with range requests, so playback can start before the download finishes.
4. Advertises the library on the LAN as a DLNA/UPnP media server.

The torrent path is the core. Catalog search and DLNA are how a title gets into the engine and how a device on the network plays it.

## Goal

Watch streams on devices on the local network using DLNA, both sides of the protocol:

| Role | Meaning | Status in this repo |
| --- | --- | --- |
| **DMS** (Digital Media Server) | This process advertises itself, answers Browse/Search, and hands devices a URL they pull with HTTP GET. | Implemented. `src/modules/dlna` wraps [`anacrolix/dms`](https://github.com/anacrolix/dms) and exposes a virtual `Movies` folder backed by the `torrents` table. |
| **DMR** (Digital Media Renderer) | A player on the LAN (TV, Kodi, another box) receives a `SetAVTransportURI` / Play from a control point and renders the stream. | Not implemented. No control point and no renderer client yet. This is the remaining half of the goal. |

Today a renderer discovers Open Streaming as a server and pulls `/stream/watch/{id}`. The intended end state is: pick a torrent here, then either let a device pull it from the DMS or push it to a DMR already on the network.

## Torrent engine

The engine lives in `src/modules/torrent_engine`. It uses [`anacrolix/torrent`](https://github.com/anacrolix/torrent) (BitTorrent over TCP and uTP, DHT, magnet and `.torrent`). The config section is still named `[webtorrent]`; the client is not the JavaScript WebTorrent library.

The client is a streaming leecher, not a seeder:

- `NoUpload = true`
- `Seed = false`
- listen port `0` (ephemeral)
- data directory from `[webtorrent] data_dir`

### Search

`GET /torrent/search?q=` does not query a tracker index directly.

1. The query goes to IMDb (`api.imdbapi.dev` by default) and returns title matches with an IMDb id and year.
2. Each id is sent to Torrentio at `{base}/brazuca/stream/movie/{imdbId}.json` (default base `https://torrentio.strem.fun`). The `brazuca` path is the Brazilian-Portuguese stream configuration currently hardcoded in the client.
3. Each stream becomes a result: title, info-hash, file index, tracker list, IMDb id, year.
4. Results are stored in an in-memory search cache (`[torrent] search_cache_ttl_minutes`, default 5). The handler returns the first 10.

Adding a torrent only works for a hash that is still in that cache. `POST /torrent/add` looks the hash up, then asks the engine to resolve it.

### Resolve and playback

`Resolve(hash)` is the hot path for both add and watch:

1. Metadata is fetched (magnet built from the info-hash and any `tracker:` sources) and cached.
2. The largest file whose path looks like a video is selected. Torrents with no video file are rejected.
3. A responsive reader is opened with a configurable readahead (`[webtorrent] readahead_mb`, default 50). Readahead is what lets a player seek and keep a buffer while rare pieces are still missing.
4. If pieces are already on disk, a hybrid reader serves completed bytes from the local file and the rest from the swarm. That is the difference between "wait for the file" and "start the movie".
5. The chosen file is stored in SQLite (`hash`, IMDb id, file index, display name, output path, size). Adding the same hash again returns the existing row.
6. The `.torrent` metainfo is written next to the data so a later restart does not depend on the search cache.

On `serve`, saved torrents are resolved in the background so the swarm is warm before a TV opens the DMS.

### Streaming the file

`GET /stream/watch/{id}` loads the row, calls `Resolve`, and passes the reader to `http.ServeContent`. That handles `Range` requests, which DLNA and most players need for seek (`DLNA.ORG_OP=01` on the resource). Response headers also set `transferMode.dlna.org: Streaming` and `contentFeatures.dlna.org`.

`GET /stream/info/{id}` returns name, hash, IMDb id, and size without opening a reader.

FFmpeg/ffprobe wrappers and a `[transcode]` config block exist under `src/modules/streaming/infrastructure/ffmpeg`. The watch path does not transcode. The DMS is started with `NoTranscode` and `NoProbe`. A TV that cannot play the container or codec will fail until transcoding is wired into this path.

### HTTP surface

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness. |
| `GET` | `/torrent/search?q=` | IMDb title search, then Torrentio streams. First 10 cached results. |
| `POST` | `/torrent/add` | JSON `{"hash":"<info-hash>"}`. Hash must be in the search cache. Returns `{"id":n}`. Times out after `[torrent] add_timeout_seconds` (default 15). |
| `GET` | `/torrent/delete/{id}` | Route is registered. Handler is still empty. |
| `GET` | `/stream/info/{id}` | Metadata for a saved torrent. |
| `GET` | `/stream/watch/{id}` | Range-capable stream of the resolved video file. This is the URL the DMS publishes. |

## DLNA

Enabled by default (`[dlna] enabled = true`). The server listens on `[dlna] port` (8200) and announces `[dlna] friendly_name` (default `Local TV`). Optional `[dlna] interface` pins SSDP to one NIC.

Browse is virtual, not a scan of `data_dir`:

- `/` contains a `Movies` container. A `Series` container appears when a stored torrent has more than one video, or a file whose name matches `SxxEyy`.
- `/Movies` lists a torrent that resolved to one video. The item title is `torrent_name`.
- `/Series` follows the torrent display path. A path that does not already start with `Series` gets that prefix. Each directory is a container. Each video is an item titled with the file name. A path segment named `sample` is not stored.
- Each item is a `object.item.videoItem`. Its resource URL is `http://<lan-ip>:<server.port>/stream/watch/{id}`, with MIME taken from the file extension (mkv, mp4, avi, mov, m4v, ts, wmv, flv, webm; anything else is advertised as `video/mp4`). A season pack has one row per video, and `watch` opens that row's file index. A single-video torrent still opens the largest video.

SSDP notify interval is 30 seconds. A control point on the LAN should see the friendly name, open Movies, and play an item by pulling the watch URL.

What is not here yet: a DMR, a control point, `SetAVTransportURI`, or any push-to-device flow. Subtitles are not published as extra resources.

## Features

- Title search through IMDb, stream list through Torrentio (`brazuca` movie configuration). Search stays off until `IMDB_BASE_URL` and `TORRENTIO_BASE_URL` are both set.
- BitTorrent streaming client with magnet parse (v1 and v2), tracker extraction, and `.torrent` persistence. `POST /torrent/add` accepts `{ "magnet": "magnet:?xt=urn:btih:..." }` and returns the id used by `/stream/watch/{id}`.
- Largest-video selection, responsive reader, readahead, hybrid local/swarm reads.
- SQLite library of added torrents, one row per video file. `GET /torrent` returns one entry per movie and one entry per season pack, with `episodes` on the pack.
- HTTP range streaming with DLNA content-feature headers.
- DLNA/UPnP DMS: SSDP announce plus a virtual Movies folder.
- Cobra CLI (`serve`, `migrate`), configuration from the environment, Docker image.

Not done: DMR / control point, transcode-on-play, delete, subtitles, a web UI.

## Tech stack

- **Go 1.25** (Docker build image is `golang:1.26-alpine`), module `github.com/c1r5/open-streaming`.
- **BitTorrent:** `github.com/anacrolix/torrent`.
- **DLNA DMS:** `github.com/anacrolix/dms` (UPnP AV browse callbacks).
- **HTTP:** stdlib `net/http` mux, CORS / recovery / logging middleware.
- **CLI:** `spf13/cobra`.
- **Storage:** SQLite via `ncruces/go-sqlite3` and GORM (`gormlite`). Default DSN is `database.db` beside the binary.
- **Catalog:** IMDb HTTP API and Torrentio HTTP API.
- **Process layout:** `catalog`, `torrent_engine`, `streaming`, `dlna`, plus `shared` config, database, and HTTP helpers.

## Run

Go toolchain on PATH. Settings come from the environment. There is no config file.

```bash
make build
make run          # build, then `<repo>/../../bin/server serve`
# or
go run ./src/cmd/server serve
```

Defaults: HTTP on `:3000`, DLNA on `:8200`, SQLite at `database.db`. Dev mode (`ENVMODE=DEV`) forces port `3001`.

```bash
make db           # migrations
docker compose up --build -d backend
```

The image only exposes `3000`. DLNA SSDP is multicast and needs host networking (or an equivalent) to be visible to TVs; the current Dockerfile does not set that.

### Config

Environment variables follow the old section names: `TORRENTIO_BASE_URL`, `TORRENTIO_USER_AGENT`, `IMDB_BASE_URL`, `WEBTORRENT_DATA_DIR`, `WEBTORRENT_READAHEAD_MB`, `WEBTORRENT_CACHE_TTL_MINUTES`, `TRANSCODE_*`, `RESOLVER_HEAD_TIMEOUT_SECONDS`, `SERVER_PORT`, `SERVER_READ_TIMEOUT_SECONDS`, `SERVER_MAX_HEADER_BYTES`, `TORRENT_SEARCH_CACHE_TTL_MINUTES`, `TORRENT_ADD_TIMEOUT_SECONDS`, `DATABASE_DSN`, `DLNA_ENABLED`, `DLNA_FRIENDLY_NAME`, `DLNA_PORT`, `DLNA_INTERFACE`. `GET /torrent/search` returns 503 until both catalog URLs are set.

## Layout

```
src/cmd/server          cobra entry: serve, migrate
src/modules/catalog     IMDb + Torrentio search
src/modules/torrent_engine
  application           add / search / cache
  infrastructure/libtorrent   client, resolve, hybrid reader, .torrent persist
src/modules/streaming   /stream/watch and /stream/info
src/modules/dlna        DMS browse of the torrents table
src/shared              config, sqlite, http helpers
```
