package subscription

import "net/http"

// DefaultVersion returns the built-in Grok CLI version used by client
// simulation.
func DefaultVersion() string {
	return CLIClientVersion
}

// ApplyClientIdentity sets the Grok CLI (grok-shell) fingerprint headers on an
// outbound request. An empty version falls back to the built-in default.
func ApplyClientIdentity(headers http.Header, version string) {
	if headers == nil {
		return
	}

	if version == "" {
		version = DefaultVersion()
	}

	headers.Set("User-Agent", "xai-grok-workspace/"+version)
	headers.Set(CLIClientVersionHeader, version)
	headers.Set(CLIClientIdentifierHeader, CLIClientIdentifier)
	headers.Set(CLITokenAuthHeader, CLITokenAuth)
}
