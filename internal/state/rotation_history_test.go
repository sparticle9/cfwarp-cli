package state_test

import (
	"errors"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
)

func TestBuildRotationFingerprint_WireGuard(t *testing.T) {
	fp, err := state.BuildRotationFingerprint(sampleAccount(), state.TransportWireGuard)
	if err != nil {
		t.Fatalf("BuildRotationFingerprint: %v", err)
	}
	if fp.Transport != state.TransportWireGuard {
		t.Fatalf("expected transport %q, got %q", state.TransportWireGuard, fp.Transport)
	}
	if fp.Fingerprint == "" || fp.IPv4Hash == "" || fp.IPv6Hash == "" {
		t.Fatalf("expected non-empty hashes, got %+v", fp)
	}
}

func TestEvaluateRotationNovelty_ByDistinctness(t *testing.T) {
	acc := sampleAccount()
	first, err := state.BuildRotationFingerprint(acc, state.TransportWireGuard)
	if err != nil {
		t.Fatalf("first fingerprint: %v", err)
	}
	history := state.RotationHistory{}
	history.RememberRotationFingerprint(first, time.Now().UTC(), 16)

	reused, err := state.BuildRotationFingerprint(acc, state.TransportWireGuard)
	if err != nil {
		t.Fatalf("reused fingerprint: %v", err)
	}
	status := state.EvaluateRotationNovelty(history, reused, state.RotationDistinctnessEither)
	if status.Qualifies {
		t.Fatalf("expected exact reuse to fail novelty check, got %+v", status)
	}
	if !status.ExactReuse || status.SeenCount == 0 {
		t.Fatalf("expected reuse metadata, got %+v", status)
	}

	acc.WARPIPV4 = "172.16.0.9/32"
	acc.WireGuard = &state.WireGuardState{IPv4: acc.WARPIPV4, IPv6: sampleAccount().WARPIPV6}
	ipv4Only, err := state.BuildRotationFingerprint(acc, state.TransportWireGuard)
	if err != nil {
		t.Fatalf("ipv4 fingerprint: %v", err)
	}
	status = state.EvaluateRotationNovelty(history, ipv4Only, state.RotationDistinctnessIPv4)
	if !status.Qualifies || !status.NewIPv4 || status.NewIPv6 {
		t.Fatalf("expected ipv4-only novelty to qualify for ipv4 policy, got %+v", status)
	}
	status = state.EvaluateRotationNovelty(history, ipv4Only, state.RotationDistinctnessBoth)
	if status.Qualifies {
		t.Fatalf("expected ipv4-only novelty to fail both policy, got %+v", status)
	}
}

func TestSaveLoadRotationHistory(t *testing.T) {
	d := tempDirs(t)
	h := state.RotationHistory{}
	h.RememberRotationFingerprint(state.RotationFingerprint{
		Transport:   state.TransportWireGuard,
		Fingerprint: "fp-1",
		IPv4Hash:    "v4-1",
		IPv6Hash:    "v6-1",
	}, time.Now().UTC().Truncate(time.Second), 16)
	if err := state.SaveRotationHistory(d, h); err != nil {
		t.Fatalf("SaveRotationHistory: %v", err)
	}
	got, err := state.LoadRotationHistory(d)
	if err != nil {
		t.Fatalf("LoadRotationHistory: %v", err)
	}
	if got.SchemaVersion != state.CurrentRotationHistorySchemaVersion {
		t.Fatalf("expected schema version %d, got %d", state.CurrentRotationHistorySchemaVersion, got.SchemaVersion)
	}
	if len(got.Entries) != 1 || got.Entries[0].Fingerprint != "fp-1" {
		t.Fatalf("unexpected history payload: %+v", got)
	}
}

func TestLoadRotationHistory_NotFound(t *testing.T) {
	d := tempDirs(t)
	got, err := state.LoadRotationHistory(d)
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if got.SchemaVersion != state.CurrentRotationHistorySchemaVersion {
		t.Fatalf("expected normalized history on not found, got %+v", got)
	}
}
