package copilot

import "net/http"

// defaultSimulationVersion mirrors the built-in Copilot Chat extension version.
const defaultSimulationVersion = "0.26.7"

// DefaultVersion returns the built-in Copilot Chat client version used by
// client simulation.
func DefaultVersion() string {
	return defaultSimulationVersion
}

// ApplyClientIdentity sets the GitHub Copilot Chat fingerprint headers on an
// outbound request. An empty version falls back to the built-in default.
//
// SetCopilotHeaders installs the full LiteLLM-style header set first so that
// every Copilot header stays consistent; the User-Agent and plugin version are
// then rewritten with the requested version.
func ApplyClientIdentity(headers http.Header, version string) {
	if headers == nil {
		return
	}

	if version == "" {
		version = defaultSimulationVersion
	}

	SetCopilotHeaders(headers)
	headers.Set(UserAgentHeader, "GitHubCopilotChat/"+version)
	headers.Set(EditorPluginVersionHeader, "copilot-chat/"+version)
}
