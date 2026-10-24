package opencode

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// defaultSimulationVersion mirrors the built-in OpenCode CLI version.
const defaultSimulationVersion = "1.18.31"

// ClientHeader identifies the OpenCode client flavor. The CLI reports "cli".
const ClientHeader = "X-Opencode-Client"

// RequestHeader carries the per-request identifier.
const RequestHeader = "X-Opencode-Request"

// ProjectHeader carries the project identifier.
const ProjectHeader = "X-Opencode-Project"

// ClientRequestIDHeader carries the client-side request identifier used for
// cache affinity. The official CLI mirrors the session ID here.
const ClientRequestIDHeader = "X-Client-Request-Id"

// defaultProject is the project value sent by the official CLI when it is not
// running inside a tracked project. OpenCode Zen accepts it as a valid value.
const defaultProject = "global"

// idCounter is the per-millisecond counter embedded in OpenCode identifiers.
// OpenCode encodes it into the low 12 bits of the descending timestamp part:
// the encoded value is ~(unixMillis<<12 | counter), so the low 12 bits become
// 0xFFF-counter. Zen's free-tier gate only accepts 0xFC0-0xFFF, i.e. counter
// in [0, 63]. Hardcode 1 so every minted identifier stays in that window.
const idCounter uint64 = 1

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// DefaultVersion returns the built-in OpenCode CLI version used by client
// simulation.
func DefaultVersion() string {
	return defaultSimulationVersion
}

// NewSessionID returns an OpenCode-style session identifier. Upstreams gate on
// the layout, which is "ses_" followed by a 12-character hex timestamp and 14
// base62 random characters. The hex part uses OpenCode's descending encoding,
// including the counter bit pattern required by OpenCode Zen.
func NewSessionID() string {
	return "ses_" + encodeDescendingID(uint64(time.Now().UnixMilli())) + randomBase62(14)
}

// SessionIDFromSeed derives a deterministic OpenCode-style session identifier
// from an arbitrary stable seed (for example a server-side trace ID or a client
// session string). The derived ID keeps the "ses_" layout and the descending
// counter bit pattern upstreams gate on while staying stable across requests of
// the same conversation.
func SessionIDFromSeed(seed string) string {
	sum := sha256.Sum256([]byte("opencode-session\x00" + seed))

	// Use 36 bits of the hash as the timestamp component. The timestamp itself
	// is not validated by the gate; only the encoded counter bits matter.
	timestampPart := binary.BigEndian.Uint64(sum[:8]) & 0xFFFFFFFFF
	encoded := encodeDescendingID(timestampPart)

	suffix := make([]byte, 14)
	for i := range suffix {
		suffix[i] = base62Alphabet[int(sum[i])%len(base62Alphabet)]
	}

	return "ses_" + encoded + string(suffix)
}

// NewRequestID returns an OpenCode-style per-request identifier: "msg_"
// followed by a 12-character hex timestamp and 14 base62 characters.
func NewRequestID() string {
	return "msg_" + encodeAscendingID(uint64(time.Now().UnixMilli())) + randomBase62(14)
}

// IsValidSessionID reports whether value has the identifier layout OpenCode
// Zen's free-tier gate accepts: "ses_" plus a 12-hex descending timestamp whose
// low 12 bits encode a counter in [0, 63], plus 14 base62 characters.
func IsValidSessionID(value string) bool {
	if !strings.HasPrefix(value, "ses_") || len(value) != len("ses_")+12+14 {
		return false
	}

	hexPart := value[len("ses_"):][:12]
	encoded, err := strconv.ParseUint(hexPart, 16, 64)
	if err != nil {
		return false
	}

	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}

	counter := encoded & 0xFFF
	if counter < 0xFC0 || counter > 0xFFF {
		return false
	}

	for _, c := range value[len("ses_")+12:] {
		if !strings.ContainsRune(base62Alphabet, c) {
			return false
		}
	}

	return true
}

// encodedIDHost renders a raw OpenCode identifier value as 12 lowercase hex
// characters (the low 48 bits, matching OpenCode's 6-byte render).
func encodedIDHost(value uint64) string {
	return fmt.Sprintf("%012x", value&0xFFFFFFFFFFFF)
}

// encodeDescendingID mirrors OpenCode's Identifier.descending():
// ~(unixMillis<<12 | counter), rendered as 6 bytes of hex.
func encodeDescendingID(timestamp uint64) string {
	return encodedIDHost(^(timestamp*4096 + idCounter))
}

// encodeAscendingID mirrors OpenCode's Identifier.ascending():
// (unixMillis<<12 | counter), rendered as 6 bytes of hex.
func encodeAscendingID(timestamp uint64) string {
	return encodedIDHost(timestamp*4096 + idCounter)
}

func randomBase62(length int) string {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failures are exceptional; fall back to a time-derived
		// value rather than returning a malformed identifier.
		for i := range buf {
			buf[i] = byte(time.Now().UnixNano() >> (i % 8 * 8))
		}
	}

	for i := range buf {
		buf[i] = base62Alphabet[int(buf[i])%len(base62Alphabet)]
	}

	return string(buf)
}

// ApplyClientIdentity sets the OpenCode CLI fingerprint headers on an outbound
// request. sessionID should be stable per conversation; an empty value
// generates a new OpenCode-style identifier. An empty version falls back to the
// built-in default.
func ApplyClientIdentity(headers http.Header, sessionID, version string) {
	if headers == nil {
		return
	}

	if version == "" {
		version = defaultSimulationVersion
	}

	if sessionID == "" {
		sessionID = NewSessionID()
	}

	headers.Set("User-Agent", "opencode/"+version)
	headers.Set(ClientHeader, "cli")
	headers.Set(ProjectHeader, defaultProject)
	headers.Set(RequestHeader, NewRequestID())
	headers.Set(ClientRequestIDHeader, sessionID)
	headers.Set(SessionHeader, sessionID)
	headers.Set(SessionIDHeader, sessionID)
	headers.Set(SessionAffinityHeader, sessionID)
}
