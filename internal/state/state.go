package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// XImportVersion is stored with each imported file hash. Bump it when a parser
// fix must be applied to files that were already imported. Same-version re-imports
// stay skipped. Rows are still merged by normalized date, so a one-time re-read
// replaces corrected days instead of duplicating them.
const XImportVersion = 2

type ImportedFile struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	ImportedAt string `json:"importedAt"`
	Rows       int    `json:"rows"`
	Version    int    `json:"version,omitempty"`
}

type YouTubeState struct {
	LastStart string `json:"lastStart,omitempty"`
	LastEnd   string `json:"lastEnd,omitempty"`
	LastRunAt string `json:"lastRunAt,omitempty"`
	ChannelID string `json:"channelId,omitempty"`
}

type XState struct {
	LastImportAt string                  `json:"lastImportAt,omitempty"`
	Files        map[string]ImportedFile `json:"files,omitempty"`
}

type State struct {
	YouTube YouTubeState `json:"youtube"`
	X       XState       `json:"x"`
}

func Load(path string) (State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{X: XState{Files: map[string]ImportedFile{}}}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	if s.X.Files == nil {
		s.X.Files = map[string]ImportedFile{}
	}
	return s, nil
}

func Save(path string, s State) error {
	if s.X.Files == nil {
		s.X.Files = map[string]ImportedFile{}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *State) MarkYouTube(channelID, start, end string, at time.Time) {
	s.YouTube.ChannelID = channelID
	s.YouTube.LastStart = start
	s.YouTube.LastEnd = end
	s.YouTube.LastRunAt = at.UTC().Format(time.RFC3339)
}

func (s *State) SeenX(sha string) bool {
	if s.X.Files == nil {
		return false
	}
	f, ok := s.X.Files[sha]
	return ok && f.Version >= XImportVersion
}

func (s *State) MarkX(f ImportedFile, at time.Time) {
	if s.X.Files == nil {
		s.X.Files = map[string]ImportedFile{}
	}
	f.Version = XImportVersion
	s.X.Files[f.SHA256] = f
	s.X.LastImportAt = at.UTC().Format(time.RFC3339)
}
