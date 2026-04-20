#!/bin/bash
# Transcription worker - watches queue directory and processes jobs
# Runs on the HOST (not in container) where whisper.cpp is installed

QUEUE_DIR="/home/ubuntu/libredesk/uploads"
WHISPER="/usr/local/bin/whisper"
MODEL="/home/ubuntu/whisper-models/ggml-base.en.bin"
DB_CONTAINER="libredesk_db"
DB_USER="libredesk"
DB_NAME="libredesk"
QUEUE_PATH="/home/ubuntu/libredesk/transcribe-queue"

mkdir -p "$QUEUE_PATH"

# Process any pending job files
process_jobs() {
    for job in "$QUEUE_PATH"/*.job; do
        [ -f "$job" ] || continue
        
        IFS='|' read -r conv_uuid media_uuid filename < "$job"
        
        echo "[$(date)] Processing: $filename (conv: $conv_uuid, media: $media_uuid)"
        
        # Read audio from uploads directory  
        AUDIO_FILE="$QUEUE_DIR/$media_uuid"
        if [ ! -f "$AUDIO_FILE" ]; then
            echo "  ERROR: Audio file not found: $AUDIO_FILE"
            rm -f "$job"
            continue
        fi
        
        # Convert to 16kHz WAV
        TMPWAV=$(mktemp /tmp/whisper-XXXXXX.wav)
        ffmpeg -i "$AUDIO_FILE" -ar 16000 -ac 1 -c:a pcm_s16le "$TMPWAV" -y -loglevel error 2>&1
        
        if [ ! -s "$TMPWAV" ]; then
            echo "  ERROR: ffmpeg conversion failed"
            rm -f "$TMPWAV" "$job"
            continue
        fi
        
        # Run whisper
        TRANSCRIPT=$($WHISPER -m "$MODEL" -f "$TMPWAV" --no-timestamps -t 2 -p 1 2>/dev/null | sed '/^$/d' | tr '\n' ' ' | sed 's/  */ /g' | sed 's/^ *//;s/ *$//')
        rm -f "$TMPWAV"
        
        if [ -z "$TRANSCRIPT" ]; then
            echo "  WARNING: Empty transcript"
            rm -f "$job"
            continue
        fi
        
        echo "  Transcript: ${TRANSCRIPT:0:100}..."
        
        # Insert as private note
        HTML_CONTENT="<p><strong>Voicemail Transcript</strong></p><p>$TRANSCRIPT</p>"
        docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c \
            "INSERT INTO conversation_messages (uuid, type, status, conversation_id, content, text_content, content_type, private, sender_id, sender_type, created_at, updated_at) SELECT gen_random_uuid(), 'outgoing', 'sent', c.id, '$HTML_CONTENT', '$TRANSCRIPT', 'html', true, 1, 'agent', NOW(), NOW() FROM conversations c WHERE c.uuid = '$conv_uuid';" 2>&1
        
        if [ $? -eq 0 ]; then
            echo "  SUCCESS: Transcript inserted"
        else
            echo "  ERROR: Failed to insert transcript"
        fi
        
        rm -f "$job"
    done
}

echo "[$(date)] Transcribe worker started, watching $QUEUE_PATH"

# Process existing jobs
process_jobs

# Watch for new jobs using inotifywait if available, otherwise poll
if command -v inotifywait &>/dev/null; then
    while true; do
        inotifywait -q -e create "$QUEUE_PATH" 2>/dev/null
        sleep 1
        process_jobs
    done
else
    # Poll every 30 seconds
    while true; do
        sleep 30
        process_jobs
    done
fi
