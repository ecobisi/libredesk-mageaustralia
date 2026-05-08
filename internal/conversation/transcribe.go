package conversation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mmodels "github.com/abhinavxd/libredesk/internal/media/models"
)

// transcribeQueueDir is where the local-whisper worker (transcribe-worker.sh)
// watches for job files. Matches v1.0.3 hard-coded path so existing host
// installations keep working without config changes.
const transcribeQueueDir = "/libredesk/transcribe-queue/"

// audioContentTypes lists MIME types that the on-ingest pipeline treats as
// transcribable audio. Mirrors v1.0.3 — covers the WAV/MP3 voicemails Twilio
// posts plus the common phone-recorder formats (m4a, aac, amr, 3gpp).
var audioContentTypes = map[string]bool{
	"audio/wav":    true,
	"audio/wave":   true,
	"audio/x-wav":  true,
	"audio/mpeg":   true,
	"audio/mp3":    true,
	"audio/ogg":    true,
	"audio/x-m4a":  true,
	"audio/mp4":    true,
	"audio/aac":    true,
	"audio/flac":   true,
	"audio/x-flac": true,
	"audio/webm":   true,
	"audio/amr":    true,
	"audio/3gpp":   true,
}

// transcribeAudioAttachments runs the configured transcription pipeline over
// each audio attachment on a freshly-ingested message. No-op when
// transcription is disabled (default) or when the message has no media.
//
// Called from ProcessIncomingMessage after attachments have been uploaded
// and the message inserted, so msg.Media has the persisted UUID/Filename
// pairs the pipeline needs. Each provider runs async so a slow Whisper or
// busy local worker doesn't stall message ingestion.
func (m *Manager) transcribeAudioAttachments(conversationUUID string, media []mmodels.Media) {
	m.lo.Info("transcribeAudioAttachments called", "conversation_uuid", conversationUUID, "media_count", len(media))

	if len(media) == 0 {
		return
	}

	aiSettings, err := m.settingsStore.GetAISettings()
	if err != nil {
		m.lo.Error("error fetching AI settings for transcription", "error", err)
		return
	}

	m.lo.Info("transcription settings", "enabled", aiSettings.TranscriptionEnabled, "provider", aiSettings.TranscriptionProvider)

	if !aiSettings.TranscriptionEnabled {
		m.lo.Info("transcription is disabled, skipping")
		return
	}

	provider := aiSettings.TranscriptionProvider
	// v1.0.3 default: empty string treated as local.
	if provider == "" {
		provider = "local"
	}

	for _, med := range media {
		ct := strings.ToLower(med.ContentType)
		m.lo.Info("checking media for transcription", "uuid", med.UUID, "content_type", ct, "is_audio", audioContentTypes[ct])
		if !audioContentTypes[ct] {
			continue
		}

		m.lo.Info("transcribing audio", "uuid", med.UUID, "filename", med.Filename, "provider", provider)

		switch provider {
		case "openai":
			go m.transcribeViaAPI(conversationUUID, med)
		case "local":
			m.transcribeViaLocal(conversationUUID, med)
		default:
			m.lo.Warn("unknown transcription provider", "provider", provider)
		}
	}
}

// transcribeViaAPI loads the audio blob and hands it to TranscribeFunc
// (wired in cmd/main.go to ai.Manager.GetOpenAIClient().TranscribeAudio).
// nil-callback or empty transcript is logged and skipped — the goal is
// graceful no-op when the API key is missing / Whisper returns silence.
func (m *Manager) transcribeViaAPI(conversationUUID string, med mmodels.Media) {
	if m.TranscribeFunc == nil {
		m.lo.Warn("transcription provider 'openai' selected but TranscribeFunc not wired; skipping", "uuid", med.UUID)
		return
	}

	audioData, err := m.mediaStore.GetBlob(med.UUID)
	if err != nil {
		m.lo.Error("error reading audio file for transcription", "error", err, "uuid", med.UUID)
		return
	}

	transcript, err := m.TranscribeFunc(audioData, med.Filename)
	if err != nil {
		m.lo.Error("error transcribing audio via API", "error", err, "uuid", med.UUID)
		return
	}

	if strings.TrimSpace(transcript) == "" {
		m.lo.Info("empty transcript from API", "uuid", med.UUID)
		return
	}

	m.insertTranscript(conversationUUID, med.Filename, transcript)
}

// transcribeViaLocal drops a job file the host-side whisper.cpp worker
// (transcribe-worker.sh) consumes via inotifywait. The worker performs
// ffmpeg conversion + whisper.cpp inference + DB insert itself; libredesk
// only owns the queue file.
//
// The job format mirrors v1.0.3: pipe-separated triple of
// conversation_uuid|media_uuid|filename. Filename is included so the
// worker can label the resulting note.
func (m *Manager) transcribeViaLocal(conversationUUID string, med mmodels.Media) {
	if err := os.MkdirAll(transcribeQueueDir, 0755); err != nil {
		m.lo.Error("error creating transcribe queue dir", "error", err, "path", transcribeQueueDir)
		return
	}

	jobContent := fmt.Sprintf("%s|%s|%s", conversationUUID, med.UUID, med.Filename)
	jobPath := filepath.Join(transcribeQueueDir, med.UUID+".job")
	if err := os.WriteFile(jobPath, []byte(jobContent), 0644); err != nil {
		m.lo.Error("error writing transcription job", "error", err, "path", jobPath)
		return
	}
	m.lo.Info("transcription job queued", "uuid", med.UUID, "job", jobPath)
}

// insertTranscript writes the transcript as a private agent note on the
// conversation. v1.0.3 used a hard-coded sender_id=1; v2 follows the same
// convention as T3y's PCI redact-activity notes and resolves the system
// user dynamically (so the note doesn't accidentally land on whichever
// agent happens to be id=1 on a fresh install).
func (m *Manager) insertTranscript(conversationUUID, filename, transcript string) {
	systemUser, err := m.userStore.GetSystemUser()
	if err != nil {
		m.lo.Error("error resolving system user for transcript note", "error", err, "conversation", conversationUUID)
		return
	}

	content := fmt.Sprintf("<p><strong>Voicemail Transcript</strong></p><p>%s</p>", transcript)

	_, err = m.db.Exec(`
		INSERT INTO conversation_messages (uuid, type, status, conversation_id, content, text_content, content_type, private, sender_id, sender_type, created_at, updated_at)
		SELECT gen_random_uuid(), 'outgoing', 'sent', c.id, $1, $2, 'html', true, $3, 'agent', NOW(), NOW()
		FROM conversations c WHERE c.uuid = $4`,
		content, transcript, systemUser.ID, conversationUUID)
	if err != nil {
		m.lo.Error("error inserting transcript", "error", err, "conversation", conversationUUID)
		return
	}

	m.lo.Info("transcript inserted", "conversation", conversationUUID, "filename", filename)
}
