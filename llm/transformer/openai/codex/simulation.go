package codex

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// DefaultVersion returns the built-in Codex CLI version used by client
// simulation.
func DefaultVersion() string {
	return codexDefaultVersion
}

// ApplyClientIdentity sets the Codex CLI fingerprint headers on an outbound
// request. sessionID should be stable per conversation; an empty value
// generates a new identifier. An empty version falls back to the built-in
// default.
func ApplyClientIdentity(headers http.Header, sessionID, version string) {
	if headers == nil {
		return
	}

	if version == "" {
		version = codexDefaultVersion
	}

	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	headers.Set("User-Agent", "codex_cli_rs/"+version)
	// Originator identifies the client family to the Codex backend; the Rust
	// CLI reports the same value it uses in its User-Agent prefix.
	headers.Set("Originator", "codex_cli_rs")
	headers.Set("Version", version)

	headers.Set(SessionHeaderHyphen, sessionID)
	headers.Set(SessionHeader, sessionID)
	headers.Set(ThreadIDHeader, sessionID)
	headers.Set("Conversation_id", sessionID)

	windowID := sessionID + ":0"
	headers.Set(WindowIDHeader, windowID)

	if headers.Get(TurnMetadataHeader) == "" {
		turnMetadata, _ := json.Marshal(TurnMetadata{
			SessionID:           sessionID,
			ThreadID:            sessionID,
			TurnID:              uuid.NewString(),
			WindowID:            windowID,
			RequestKind:         "turn",
			ThreadSource:        "user",
			Sandbox:             "none",
			TurnStartedAtUnixMS: time.Now().UnixMilli(),
		})
		headers.Set(TurnMetadataHeader, string(turnMetadata))
	}

	if headers.Get(ClientRequestIDHeader) == "" {
		headers.Set(ClientRequestIDHeader, uuid.NewString())
	}

	if headers.Get(BetaFeaturesHeader) == "" {
		headers.Set(BetaFeaturesHeader, fabricatedBetaFeatures)
	}
}
