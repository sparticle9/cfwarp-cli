package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
)

func rotationDistinctness(sett state.Settings) string {
	if sett.Rotation == nil {
		return state.RotationDistinctnessEither
	}
	if sett.Rotation.Distinctness == "" {
		return state.RotationDistinctnessEither
	}
	return sett.Rotation.Distinctness
}

func rotationHistorySize(sett state.Settings) int {
	if sett.Rotation != nil && sett.Rotation.HistorySize > 0 {
		return sett.Rotation.HistorySize
	}
	return state.DefaultRotationHistorySize
}

func rememberCurrentRotationAccount(dirs state.Dirs, sett state.Settings) (*state.RotationNovelty, error) {
	acc, err := state.LoadAccount(dirs)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	status, err := ensureRotationAccount(dirs, acc, sett)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func ensureRotationAccount(dirs state.Dirs, acc state.AccountState, sett state.Settings) (state.RotationNovelty, error) {
	history, err := state.LoadRotationHistory(dirs)
	if err != nil && !errors.Is(err, state.ErrNotFound) {
		return state.RotationNovelty{}, fmt.Errorf("load rotation history: %w", err)
	}
	fp, err := state.BuildRotationFingerprint(acc, sett.Transport)
	if err != nil {
		return state.RotationNovelty{}, err
	}
	status := state.EvaluateRotationNovelty(history, fp, rotationDistinctness(sett))
	if status.SeenCount == 0 {
		history.RememberRotationFingerprint(fp, time.Now().UTC(), rotationHistorySize(sett))
		status.HistoryEntries = len(history.Entries)
		if err := state.SaveRotationHistory(dirs, history); err != nil {
			return state.RotationNovelty{}, fmt.Errorf("save rotation history: %w", err)
		}
	}
	if status.HistoryEntries == 0 {
		status.HistoryEntries = len(history.Entries)
	}
	return status, nil
}

func rememberRotationAccount(dirs state.Dirs, acc state.AccountState, sett state.Settings, observedAt time.Time) (state.RotationNovelty, error) {
	history, err := state.LoadRotationHistory(dirs)
	if err != nil && !errors.Is(err, state.ErrNotFound) {
		return state.RotationNovelty{}, fmt.Errorf("load rotation history: %w", err)
	}
	fp, err := state.BuildRotationFingerprint(acc, sett.Transport)
	if err != nil {
		return state.RotationNovelty{}, err
	}
	status := state.EvaluateRotationNovelty(history, fp, rotationDistinctness(sett))
	history.RememberRotationFingerprint(fp, observedAt, rotationHistorySize(sett))
	status.HistoryEntries = len(history.Entries)
	if err := state.SaveRotationHistory(dirs, history); err != nil {
		return state.RotationNovelty{}, fmt.Errorf("save rotation history: %w", err)
	}
	return status, nil
}

func formatRotationNovelty(status state.RotationNovelty) string {
	return fmt.Sprintf(
		"rotation_memory distinctness=%s qualifies=%t exact_reuse=%t seen_count=%d new_ipv4=%t new_ipv6=%t",
		status.Distinctness,
		status.Qualifies,
		status.ExactReuse,
		status.SeenCount,
		status.NewIPv4,
		status.NewIPv6,
	)
}
