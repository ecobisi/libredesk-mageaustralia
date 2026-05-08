#!/bin/bash
# T3v: Local-whisper.cpp transcription worker for Libredesk voicemails.
#
# Watches QUEUE_DIR for job files dropped by internal/conversation/transcribe.go
# (transcribeViaLocal). Each job file holds a pipe-separated triple of
# {conversation_uuid}|{media_uuid}|{filename}. The worker converts the audio
# blob to 16kHz mono WAV via ffmpeg, runs whisper.cpp inference, then inserts
# the transcript as a private agent note via psql.
#
# Operational tool: ship-as-is, run on the host (not inside the container)
# because whisper.cpp + ffmpeg are heavy native binaries we don't want in the
# image. See docs/voice-transcription.md for the systemd unit + setup steps.
#
# Mirrors v1.0.3's worker; paths are intentionally hardcoded to the same
# locations the production deployment uses (QUEUE_DIR is bind-mounted into
# the container at /libredesk/transcribe-queue/, so the Go side and this
# script see the same directory). Adjust the four config vars at the top
# for your install if needed.

QUEUE_DIR="/home/ubuntu/libredesk/transcribe-queue"
MODEL="/home/ubuntu/whisper-models/ggml-base.en.bin"
UPLOAD_DIR="/home/ubuntu/libredesk/uploads"
DB_CONTAINER="libredesk_db"
DB_USER="libredesk"
DB_NAME="libredesk"

mkdir -p "$QUEUE_DIR"

echo "Transcription worker started, watching $QUEUE_DIR"

inotifywait -m -e create --format '%f' "$QUEUE_DIR" 2>/dev/null | while read jobfile; do
    sleep 0.5  # Let file finish writing
    JOBPATH="$QUEUE_DIR/$jobfile"
    [ ! -f "$JOBPATH" ] && continue

    # Parse job: conversation_uuid|media_uuid|filename
    IFS='|' read -r CONV_UUID MEDIA_UUID FILENAME < "$JOBPATH"

    echo "Transcribing: $FILENAME (media: $MEDIA_UUID, conv: $CONV_UUID)"

    INPUT="$UPLOAD_DIR/$MEDIA_UUID"
    if [ ! -f "$INPUT" ]; then
        echo "Error: file not found $INPUT"
        rm -f "$JOBPATH"
        continue
    fi

    # Convert to 16kHz mono WAV (whisper.cpp requirement)
    TMPWAV=$(mktemp /tmp/whisper-XXXXXX.wav)
    ffmpeg -i "$INPUT" -ar 16000 -ac 1 -c:a pcm_s16le "$TMPWAV" -y -loglevel error 2>&1

    if [ ! -s "$TMPWAV" ]; then
        echo "Error: ffmpeg conversion failed for $FILENAME"
        rm -f "$TMPWAV" "$JOBPATH"
        continue
    fi

    # Transcribe with whisper.cpp
    TRANSCRIPT=$(whisper -m "$MODEL" -f "$TMPWAV" --no-timestamps -t 2 -p 1 2>/dev/null | sed '/^$/d' | tr -s ' ')
    rm -f "$TMPWAV"

    if [ -z "$TRANSCRIPT" ] || [ "$TRANSCRIPT" = "[BLANK_AUDIO]" ]; then
        echo "No transcript content for $FILENAME"
        rm -f "$JOBPATH"
        continue
    fi

    echo "Transcript: $TRANSCRIPT"

    # Insert as private note via psql. Sender is hardcoded to user id 1
    # (system user); if your install has reassigned id 1 you'll want to
    # join on (SELECT id FROM users WHERE type='system' LIMIT 1) instead.
    ESCAPED=$(echo "$TRANSCRIPT" | sed "s/'/''/g")
    CONTENT="<p><strong>Voicemail Transcript</strong></p><p>$ESCAPED</p>"

    docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "
        INSERT INTO conversation_messages (uuid, type, status, conversation_id, content, text_content, content_type, private, sender_id, sender_type, created_at, updated_at)
        SELECT gen_random_uuid(), 'outgoing', 'sent', c.id, '$CONTENT', '$ESCAPED', 'html', true, 1, 'agent', NOW(), NOW()
        FROM conversations c WHERE c.uuid = '$CONV_UUID';
    " 2>&1

    echo "Transcript inserted for conversation $CONV_UUID"
    rm -f "$JOBPATH"
done
