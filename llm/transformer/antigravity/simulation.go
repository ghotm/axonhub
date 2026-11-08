package antigravity

import "net/http"

// Client identity headers reported by the IDE.
const (
	ClientNameHeader     = "X-Client-Name"
	ClientVersionHeader  = "X-Client-Version"
	APIClientHeader      = "X-Goog-Api-Client"
	ClientMetadataHeader = "Client-Metadata"
)

// DefaultVersion returns the built-in Antigravity IDE version used by client
// simulation when the caller does not override it.
func DefaultVersion() string {
	return UserAgentVersionFallback
}

// ApplyClientIdentity sets the Antigravity IDE fingerprint headers on an
// outbound request. An empty version falls back to the built-in default.
//
// Unlike SetClientHeaders it uses the explicitly requested version instead of
// the process-wide resolved one, so the simulated identity does not drift when
// the background version check updates the real client version.
func ApplyClientIdentity(headers http.Header, version string) {
	if headers == nil {
		return
	}

	if version == "" {
		version = DefaultVersion()
	}

	headers.Set("User-Agent", "antigravity/"+version+" windows/amd64")
	headers.Set(ClientNameHeader, "antigravity")
	headers.Set(ClientVersionHeader, version)
	headers.Set(APIClientHeader, ApiClient)
	headers.Set(ClientMetadataHeader, ClientMetadata)
}
