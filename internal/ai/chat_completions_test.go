package ai

import (
	"reflect"
	"testing"
)

// Lock in the wire-shape switching behaviour of buildChatMessages so a
// future refactor can't silently break the multimodal vs text-only
// distinction (OpenAI rejects content-as-array for text-only turns
// inconsistently across versions).
func TestBuildChatMessages_TextOnly(t *testing.T) {
	got := buildChatMessages(PromptPayload{
		SystemPrompt: "you are helpful",
		UserPrompt:   "hello",
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	system, ok := got[0].(map[string]string)
	if !ok || system["role"] != "system" || system["content"] != "you are helpful" {
		t.Errorf("system message wrong shape: %+v", got[0])
	}
	// Text-only payloads keep the cheap content-as-string shape.
	user, ok := got[1].(map[string]string)
	if !ok {
		t.Errorf("text-only user message should be map[string]string, got %T", got[1])
	}
	if user["role"] != "user" || user["content"] != "hello" {
		t.Errorf("user message wrong: %+v", got[1])
	}
}

func TestBuildChatMessages_Multimodal(t *testing.T) {
	got := buildChatMessages(PromptPayload{
		SystemPrompt: "you are helpful",
		UserPrompt:   "what is in this picture",
		Images: []ImageContent{
			{URL: "data:image/png;base64,xxx", Filename: "screenshot.png"},
			{URL: "https://example.com/img.jpg", Filename: "remote.jpg"},
		},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	// Multimodal must use the content-as-array shape — it's the only
	// wire form that lets image_url parts coexist with text.
	user, ok := got[1].(map[string]interface{})
	if !ok {
		t.Fatalf("multimodal user message should be map[string]interface{}, got %T", got[1])
	}
	if user["role"] != "user" {
		t.Errorf("user role wrong: %+v", user)
	}
	parts, ok := user["content"].([]map[string]interface{})
	if !ok {
		t.Fatalf("content should be []map, got %T", user["content"])
	}
	if len(parts) != 3 { // 1 text + 2 images
		t.Fatalf("expected 3 content parts, got %d", len(parts))
	}
	if parts[0]["type"] != "text" || parts[0]["text"] != "what is in this picture" {
		t.Errorf("first part should be text: %+v", parts[0])
	}
	for i, p := range parts[1:] {
		if p["type"] != "image_url" {
			t.Errorf("part %d should be image_url, got %v", i+1, p["type"])
		}
		imgURL, ok := p["image_url"].(map[string]string)
		if !ok {
			t.Fatalf("image_url should be map[string]string, got %T", p["image_url"])
		}
		// "low" detail is load-bearing — keeps token cost flat at
		// ~85 tokens regardless of image dimensions.
		if imgURL["detail"] != "low" {
			t.Errorf("part %d detail tier should be 'low', got %q", i+1, imgURL["detail"])
		}
	}
}

// Wire shape compatibility: the OpenAI chat-completions API treats the
// system message + user message as the canonical 2-element array. Make
// sure we don't accidentally drop the system-prompt slot when the user
// prompt is empty.
func TestBuildChatMessages_EmptyUserPrompt(t *testing.T) {
	got := buildChatMessages(PromptPayload{
		SystemPrompt: "system",
		UserPrompt:   "",
	})
	want := []interface{}{
		map[string]string{"role": "system", "content": "system"},
		map[string]string{"role": "user", "content": ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
