package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Profile is the single source of truth for a project. `init` writes it after
// detection; every generator is a pure function of it, so the whole setup is
// regenerable and reviewable as a committed file.
type Profile struct {
	Name      string `json:"name"`
	Stack     string `json:"stack"`            // django | fastapi | go | java | node
	Framework string `json:"framework"`        // optional sub-type (next, express, ...)
	Port      int    `json:"port"`             // container port the app listens on
	Registry  string `json:"registry"`         // e.g. ghcr.io/owner/name
	Target    string `json:"target"`           // hetzner-vps
	NeedsDB   bool   `json:"needs_db"`          // add a postgres service to compose

	// Stack-specific knobs consumed by the Dockerfile templates.
	WSGIModule    string `json:"wsgi_module,omitempty"`     // django, e.g. myproj.wsgi
	AppModule     string `json:"app_module,omitempty"`      // fastapi, e.g. main:app
	BinaryName    string `json:"binary_name,omitempty"`     // go
	JavaBuildTool string `json:"java_build_tool,omitempty"` // maven | gradle
}

const profilePath = ".devopsify.json"

func (p *Profile) Save() error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profilePath, append(b, '\n'), 0o644)
}

func LoadProfile() (*Profile, error) {
	b, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no %s found — run `devopsify init` first", profilePath)
		}
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", profilePath, err)
	}
	return &p, nil
}
