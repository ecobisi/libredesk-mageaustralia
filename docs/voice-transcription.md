# Voice Transcription Setup (Local whisper.cpp)

LibreDesk can automatically transcribe audio attachments (voicemails, voice notes) into text using a local [whisper.cpp](https://github.com/ggerganov/whisper.cpp) installation. Transcripts appear as private notes on the conversation.

The cloud alternative — OpenAI's Whisper API — is also supported and only requires a Whisper-capable OpenAI API key configured under Admin → AI. The cloud path needs zero host setup and costs ~$0.006/minute. Use this guide if you'd rather run everything on the host (free, private, offline).

## Requirements

- Linux host (tested on Ubuntu 22.04+ / ARM64 and x86_64)
- ~500MB disk space (whisper.cpp build + model)
- ~250MB RAM during transcription
- `ffmpeg` installed
- `inotifywait` (from `inotify-tools`)
- PostgreSQL client (`psql`) accessible via `docker exec`

## 1. Install dependencies

```bash
sudo apt update
sudo apt install -y ffmpeg inotify-tools build-essential
```

## 2. Build whisper.cpp

```bash
cd /home/ubuntu
git clone https://github.com/ggerganov/whisper.cpp.git
cd whisper.cpp
cmake -B build
cmake --build build -j$(nproc)
sudo cp build/bin/whisper-cli /usr/local/bin/whisper
```

## 3. Download a model

The `base.en` model is recommended for voicemail transcription — it's fast and accurate for English:

```bash
mkdir -p /home/ubuntu/whisper-models
cd /home/ubuntu/whisper.cpp
bash models/download-ggml-model.sh base.en
cp models/ggml-base.en.bin /home/ubuntu/whisper-models/
```

Other model options:

| Model | Size | RAM | Speed | Notes |
|-------|------|-----|-------|-------|
| `tiny.en` | 75MB | ~125MB | Fastest | Lower accuracy |
| `base.en` | 142MB | ~210MB | Fast | Good balance (recommended) |
| `small.en` | 466MB | ~600MB | Moderate | Higher accuracy |

## 4. Run the worker

The repo ships `transcribe-worker.sh` — copy it to your install root, adjust the four config vars at the top if your install paths differ, then make it executable:

```bash
cp transcribe-worker.sh /home/ubuntu/libredesk/transcribe-worker.sh
chmod +x /home/ubuntu/libredesk/transcribe-worker.sh
```

The default config matches a docker-compose install with the queue dir bind-mounted at `/home/ubuntu/libredesk/transcribe-queue` (host) ↔ `/libredesk/transcribe-queue` (container). The Go side writes job files into the container path; the worker watches the host path.

## 5. Create the systemd service

Create `/etc/systemd/system/libredesk-transcribe.service`:

```ini
[Unit]
Description=LibreDesk Voice Transcription Worker
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=ubuntu
ExecStart=/home/ubuntu/libredesk/transcribe-worker.sh
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable libredesk-transcribe
sudo systemctl start libredesk-transcribe
```

## 6. Enable in LibreDesk

1. Go to **Admin → AI**.
2. Toggle **Enable transcription** on.
3. Select **Local whisper.cpp (self-hosted)** as the provider.
4. Click **Save**.

## Verification

Check that the worker is running:

```bash
sudo systemctl status libredesk-transcribe
```

View worker logs:

```bash
sudo journalctl -u libredesk-transcribe -f
```

Send a test voicemail or audio file to your support inbox. The transcription should appear as a private note within a few seconds.

## Troubleshooting

**Worker not starting**: Check that `inotifywait` is installed (`apt install inotify-tools`).

**Transcription fails**: Ensure `ffmpeg` and `whisper` are in the PATH. Test manually:

```bash
whisper -m /home/ubuntu/whisper-models/ggml-base.en.bin -f /path/to/test.wav --no-timestamps
```

**Empty transcripts**: The audio may be too short or silent. Check the original file.

**DB insert fails**: Verify the Docker container name matches (`libredesk_db`) and the system user ID is `1`:

```bash
docker exec libredesk_db psql -U libredesk -d libredesk -c "SELECT id, first_name FROM users WHERE id = 1;"
```

If id=1 is a real agent on your install (older deployments may have done this), edit the worker's INSERT to look up the system user dynamically:

```sql
SELECT id FROM users WHERE type='system' LIMIT 1
```

The Go-side OpenAI path already does this lookup via `userStore.GetSystemUser()`; only the shell worker hardcodes id=1 to keep the script simple.
