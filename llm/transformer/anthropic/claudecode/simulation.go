package claudecode

import (
	"fmt"
	"net/http"
)

// SessionHeader is the Claude Code CLI conversation/session identifier header.
const SessionHeader = "X-Claude-Code-Session-Id"

// defaultSimulationVersion mirrors the built-in Claude Code CLI version.
const defaultSimulationVersion = "2.1.170"

// DefaultVersion returns the built-in Claude Code CLI version used by client
// simulation.
func DefaultVersion() string {
	return defaultSimulationVersion
}

// ApplyClientIdentity sets the Claude Code CLI fingerprint headers on an
// outbound request. An empty version falls back to the built-in default. The
// caller is responsible for setting the session header when a stable session
// identifier is available.
func ApplyClientIdentity(headers http.Header, version string) {
	if headers == nil {
		return
	}

	if version == "" {
		version = defaultSimulationVersion
	}

	headers.Set("User-Agent", fmt.Sprintf("claude-cli/%s (external, cli)", version))

	for _, entry := range claudeCodeHeaders {
		headers.Set(entry[0], entry[1])
	}
}
