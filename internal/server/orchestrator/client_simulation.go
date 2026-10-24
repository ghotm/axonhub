package orchestrator

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/opencode"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// applyClientSimulation creates a middleware that overlays the CLI fingerprint
// configured on the current channel.
//
// Placement matters: it runs after the user-agent pass-through middleware so
// an explicit simulation always wins over the generic UA policy, and before the
// header-override middleware so operators can still override individual
// headers on top of the simulated identity.
func applyClientSimulation(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("client-simulation", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil || channel.Settings == nil || channel.Settings.ClientSimulation == nil {
			return request, nil
		}

		simulation := channel.Settings.ClientSimulation
		if simulation.Type == "" || simulation.Type == objects.ClientSimulationTypeNone {
			return request, nil
		}

		if request.Headers == nil {
			request.Headers = make(http.Header)
		}

		sessionID := resolveClientSimulationSessionID(ctx, request.Headers)

		switch simulation.Type {
		case objects.ClientSimulationTypeCodex:
			prepareClientSimulationHeaders(request.Headers)
			codex.ScrubClientFingerprintHeaders(request.Headers)
			codex.ApplyClientIdentity(request.Headers, sessionID, simulation.Version)
		case objects.ClientSimulationTypeClaudeCode:
			prepareClientSimulationHeaders(request.Headers)
			claudecode.ApplyClientIdentity(request.Headers, simulation.Version)

			if sessionID != "" {
				request.Headers.Set(claudecode.SessionHeader, sessionID)
			}

			if request.APIFormat == string(llm.APIFormatAnthropicMessage) {
				if request.Query == nil {
					request.Query = url.Values{}
				}

				request.Query.Set("beta", "true")
			}
		case objects.ClientSimulationTypeOpenCode:
			prepareClientSimulationHeaders(request.Headers)
			opencode.ApplyClientIdentity(request.Headers, normalizeOpenCodeSessionID(sessionID), simulation.Version)
		}

		return request, nil
	})
}

// resolveClientSimulationSessionID picks the most stable conversation
// identifier available: a client-provided CLI session header first, then the
// shared session stored on the context, and finally an empty value which lets
// the profile generate a fresh identifier.
func resolveClientSimulationSessionID(ctx context.Context, headers http.Header) string {
	for _, header := range []string{
		claudecode.SessionHeader,
		codex.SessionHeaderHyphen,
		codex.SessionHeader,
		opencode.SessionHeader,
		opencode.SessionIDHeader,
		opencode.SessionAffinityHeader,
	} {
		if value := strings.TrimSpace(headers.Get(header)); value != "" {
			return value
		}
	}

	if sessionID, ok := shared.GetSessionID(ctx); ok {
		if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// prepareClientSimulationHeaders removes session identifiers and attribution
// headers that belong to a different CLI so the simulated fingerprint does not
// mix identities. Protocol headers (content type, accept, auth, provider
// version headers) are intentionally left untouched.
func prepareClientSimulationHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	for _, name := range []string{
		codex.SessionHeader,
		codex.SessionHeaderHyphen,
		codex.ThreadIDHeader,
		codex.WindowIDHeader,
		codex.TurnMetadataHeader,
		codex.TurnStateHeader,
		codex.ClientRequestIDHeader,
		codex.BetaFeaturesHeader,
		codex.ResponsesLiteHeader,
		"Conversation_id",
		"Originator",
		"Version",
		claudecode.SessionHeader,
		opencode.SessionHeader,
		opencode.SessionIDHeader,
		opencode.SessionAffinityHeader,
		opencode.RequestHeader,
		opencode.ClientHeader,
		opencode.ProjectHeader,
		opencode.ClientRequestIDHeader,
		"X-Title",
	} {
		headers.Del(name)
	}
}

// normalizeOpenCodeSessionID preserves an existing OpenCode session ID and
// derives an OpenCode-shaped one from any other stable identifier. Upstreams
// gate on the "ses_<timestamp><random>" layout, so trace IDs and UUIDs must be
// converted rather than copied verbatim.
func normalizeOpenCodeSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}

	if opencode.IsValidSessionID(sessionID) {
		return sessionID
	}

	return opencode.SessionIDFromSeed(sessionID)
}
