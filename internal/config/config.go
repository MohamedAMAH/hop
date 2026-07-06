/*
Package config stores hop's user-editable settings: the per-machine project
path map, transport choice, and hand-off mode. It lives in hop's own dir,
never under ~/.claude.
*/
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

/*
Project holds one project's cross-machine settings.
*/
type Project struct {
	Paths           map[string]string `json:"paths"`           // machine name -> absolute path
	Transport       string            `json:"transport"`       // e.g. "folder"
	TransportConfig map[string]string `json:"transportConfig"` // e.g. {"dir": "..."}
	Handoff         string            `json:"handoff"`         // "manual" | "auto"
}

/*
Config is the top-level hop configuration for this machine.
*/
type Config struct {
	Machine  string             `json:"machine"`
	Projects map[string]Project `json:"projects"`
}

/*
PathFor returns the absolute project path for a machine, if recorded.
*/
func (c Config) PathFor(projectID, machine string) (string, bool) {
	p, ok := c.Projects[projectID]
	if !ok {
		return "", false
	}
	path, ok := p.Paths[machine]
	return path, ok
}

/*
DefaultDir returns hop's config directory (~/.config/hop or the OS default).
*/
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hop"), nil
}

/*
Load reads the config; a missing file yields an initialized empty Config.
*/
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{Projects: map[string]Project{}}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if c.Projects == nil {
		c.Projects = map[string]Project{}
	}
	return c, nil
}

/*
Save writes the config atomically (temp + rename).
*/
func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "config.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
