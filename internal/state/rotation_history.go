package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	CurrentRotationHistorySchemaVersion = 1
	DefaultRotationHistorySize          = 128

	RotationDistinctnessEither = "either"
	RotationDistinctnessIPv4   = "ipv4"
	RotationDistinctnessIPv6   = "ipv6"
	RotationDistinctnessBoth   = "both"
)

// RotationFingerprint is the hashed identity of one assigned address set for a transport.
type RotationFingerprint struct {
	Transport   string `json:"transport,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	IPv4Hash    string `json:"ipv4_hash,omitempty"`
	IPv6Hash    string `json:"ipv6_hash,omitempty"`
}

// RotationHistoryEntry stores one previously-seen hashed address assignment.
type RotationHistoryEntry struct {
	Transport   string    `json:"transport,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	IPv4Hash    string    `json:"ipv4_hash,omitempty"`
	IPv6Hash    string    `json:"ipv6_hash,omitempty"`
	FirstSeenAt time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt  time.Time `json:"last_seen_at,omitempty"`
	SeenCount   int       `json:"seen_count,omitempty"`
}

// RotationHistory stores durable hashed memory of observed address assignments.
type RotationHistory struct {
	SchemaVersion int                    `json:"schema_version,omitempty"`
	Entries       []RotationHistoryEntry `json:"entries,omitempty"`
}

// RotationNovelty summarizes whether a newly observed address assignment was novel.
type RotationNovelty struct {
	Transport      string `json:"transport,omitempty"`
	Distinctness   string `json:"distinctness,omitempty"`
	Fingerprint    string `json:"fingerprint,omitempty"`
	HistoryEntries int    `json:"history_entries,omitempty"`
	SeenCount      int    `json:"seen_count,omitempty"`
	ExactReuse     bool   `json:"exact_reuse,omitempty"`
	NewFingerprint bool   `json:"new_fingerprint,omitempty"`
	NewIPv4        bool   `json:"new_ipv4,omitempty"`
	NewIPv6        bool   `json:"new_ipv6,omitempty"`
	Qualifies      bool   `json:"qualifies,omitempty"`
}

func (h *RotationHistory) Normalize() {
	if h.SchemaVersion == 0 {
		h.SchemaVersion = CurrentRotationHistorySchemaVersion
	}
	for i := range h.Entries {
		h.Entries[i].Transport = strings.ToLower(strings.TrimSpace(h.Entries[i].Transport))
		if h.Entries[i].SeenCount <= 0 {
			h.Entries[i].SeenCount = 1
		}
	}
}

// BuildRotationFingerprint hashes the currently assigned addresses for transport.
func BuildRotationFingerprint(acc AccountState, transport string) (RotationFingerprint, error) {
	acc.Normalize()
	transport = strings.ToLower(strings.TrimSpace(transport))
	var ipv4, ipv6 string
	switch transport {
	case TransportWireGuard:
		if acc.WireGuard != nil {
			ipv4 = acc.WireGuard.IPv4
			ipv6 = acc.WireGuard.IPv6
		} else {
			ipv4 = acc.WARPIPV4
			ipv6 = acc.WARPIPV6
		}
	case TransportMasque:
		if acc.Masque == nil {
			return RotationFingerprint{}, fmt.Errorf("account has no MASQUE state")
		}
		ipv4 = acc.Masque.IPv4
		ipv6 = acc.Masque.IPv6
	default:
		return RotationFingerprint{}, fmt.Errorf("unsupported transport %q", transport)
	}
	ipv4 = normalizeAddressValue(ipv4)
	ipv6 = normalizeAddressValue(ipv6)
	if ipv4 == "" && ipv6 == "" {
		return RotationFingerprint{}, fmt.Errorf("transport %s has no assigned addresses", transport)
	}
	fp := RotationFingerprint{
		Transport:   transport,
		Fingerprint: hashRotationValue("transport|" + transport + "|ipv4|" + ipv4 + "|ipv6|" + ipv6),
	}
	if ipv4 != "" {
		fp.IPv4Hash = hashRotationValue("ipv4|" + transport + "|" + ipv4)
	}
	if ipv6 != "" {
		fp.IPv6Hash = hashRotationValue("ipv6|" + transport + "|" + ipv6)
	}
	return fp, nil
}

func normalizeAddressValue(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func hashRotationValue(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

// NormalizeRotationDistinctness canonicalizes the distinctness policy.
func NormalizeRotationDistinctness(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", RotationDistinctnessEither:
		return RotationDistinctnessEither
	case RotationDistinctnessIPv4:
		return RotationDistinctnessIPv4
	case RotationDistinctnessIPv6:
		return RotationDistinctnessIPv6
	case RotationDistinctnessBoth:
		return RotationDistinctnessBoth
	default:
		return strings.ToLower(strings.TrimSpace(v))
	}
}

// EvaluateRotationNovelty checks whether fp is novel against prior hashed memory.
func EvaluateRotationNovelty(history RotationHistory, fp RotationFingerprint, distinctness string) RotationNovelty {
	history.Normalize()
	distinctness = NormalizeRotationDistinctness(distinctness)
	if distinctness == "" {
		distinctness = RotationDistinctnessEither
	}
	seenCount := 0
	seenIPv4 := false
	seenIPv6 := false
	for _, entry := range history.Entries {
		if entry.Transport != fp.Transport {
			continue
		}
		if fp.Fingerprint != "" && entry.Fingerprint == fp.Fingerprint {
			seenCount += maxInt(entry.SeenCount, 1)
		}
		if fp.IPv4Hash != "" && entry.IPv4Hash == fp.IPv4Hash {
			seenIPv4 = true
		}
		if fp.IPv6Hash != "" && entry.IPv6Hash == fp.IPv6Hash {
			seenIPv6 = true
		}
	}
	status := RotationNovelty{
		Transport:      fp.Transport,
		Distinctness:   distinctness,
		Fingerprint:    fp.Fingerprint,
		HistoryEntries: len(history.Entries),
		SeenCount:      seenCount,
		ExactReuse:     seenCount > 0,
		NewFingerprint: seenCount == 0,
		NewIPv4:        fp.IPv4Hash != "" && !seenIPv4,
		NewIPv6:        fp.IPv6Hash != "" && !seenIPv6,
	}
	switch distinctness {
	case RotationDistinctnessIPv4:
		status.Qualifies = status.NewIPv4
	case RotationDistinctnessIPv6:
		status.Qualifies = status.NewIPv6
	case RotationDistinctnessBoth:
		status.Qualifies = status.NewIPv4 && status.NewIPv6
	default:
		status.Qualifies = status.NewIPv4 || status.NewIPv6 || status.NewFingerprint
	}
	return status
}

// RememberRotationFingerprint records fp in history and trims to maxEntries.
func (h *RotationHistory) RememberRotationFingerprint(fp RotationFingerprint, observedAt time.Time, maxEntries int) {
	h.Normalize()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	for i := range h.Entries {
		if h.Entries[i].Transport == fp.Transport && h.Entries[i].Fingerprint == fp.Fingerprint {
			if h.Entries[i].FirstSeenAt.IsZero() {
				h.Entries[i].FirstSeenAt = observedAt
			}
			h.Entries[i].LastSeenAt = observedAt
			h.Entries[i].SeenCount = maxInt(h.Entries[i].SeenCount, 1) + 1
			return
		}
	}
	entry := RotationHistoryEntry{
		Transport:   fp.Transport,
		Fingerprint: fp.Fingerprint,
		IPv4Hash:    fp.IPv4Hash,
		IPv6Hash:    fp.IPv6Hash,
		FirstSeenAt: observedAt,
		LastSeenAt:  observedAt,
		SeenCount:   1,
	}
	h.Entries = append([]RotationHistoryEntry{entry}, h.Entries...)
	if maxEntries <= 0 {
		maxEntries = DefaultRotationHistorySize
	}
	if len(h.Entries) > maxEntries {
		h.Entries = h.Entries[:maxEntries]
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
