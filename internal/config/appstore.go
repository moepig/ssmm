package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/sshconfig"
)

type AppStore struct {
	Paths          Paths
	SSMMExecutable string
}

func NewAppStore(paths Paths, ssmmExecutable string) AppStore {
	return AppStore{Paths: paths, SSMMExecutable: ssmmExecutable}
}

func (s AppStore) Read(context.Context) (app.SettingsSnapshot, error) {
	settings, err := ReadLenient(s.Paths.ConfigFile)
	if err != nil {
		return app.SettingsSnapshot{}, err
	}
	return toSnapshot(settings, s.Paths.ConfigFile), nil
}

func (s AppStore) ReadForUpdate(ctx context.Context) (app.SettingsDraft, error) {
	snapshot, err := s.Read(ctx)
	if err != nil {
		return app.SettingsDraft{}, err
	}
	original := map[string]app.FileState{}
	for _, path := range []string{s.Paths.ConfigFile, s.Paths.SSHManagedFile, s.Paths.SSHConfigFile} {
		state, err := readFileState(path)
		if err != nil {
			return app.SettingsDraft{}, err
		}
		original[path] = state
	}
	return app.SettingsDraft{Snapshot: snapshot, Original: original}, nil
}

func (s AppStore) Commit(ctx context.Context, update app.SettingsUpdate) (app.SaveReport, error) {
	if err := ctx.Err(); err != nil {
		return app.SaveReport{}, err
	}
	unlock, err := acquireLock(s.Paths.LockFile)
	if err != nil {
		return app.SaveReport{}, err
	}
	defer unlock()
	for path, expected := range update.Original {
		current, err := readFileState(path)
		if err != nil {
			return app.SaveReport{}, err
		}
		if !sameFileState(current, expected) {
			return app.SaveReport{}, fmt.Errorf("settings changed while editing: %s", path)
		}
	}
	settings := fromSnapshot(update.Snapshot)
	if err := settings.Validate(filepath.Dir(s.Paths.ConfigFile)); err != nil {
		return app.SaveReport{}, fmt.Errorf("validate settings: %w", err)
	}
	data, err := Encode(settings)
	if err != nil {
		return app.SaveReport{}, err
	}
	profiles := make([]sshconfig.Profile, 0, len(update.Snapshot.Profiles))
	for name, profile := range update.Snapshot.Profiles {
		profiles = append(profiles, sshconfig.Profile{Name: name, User: profile.SSH.User, IdentityFile: profile.SSH.IdentityFile, Port: profile.SSH.Port, Integration: profile.Integration})
	}
	managed, err := sshconfig.RenderManaged(profiles, s.SSMMExecutable)
	if err != nil {
		return app.SaveReport{}, fmt.Errorf("generate %s: %w", s.Paths.SSHManagedFile, err)
	}
	existing, err := os.ReadFile(s.Paths.SSHConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return app.SaveReport{}, err
	}
	integration := false
	for _, profile := range update.Snapshot.Profiles {
		if profile.Integration {
			integration = true
			break
		}
	}
	updated := sshconfig.RemoveInclude(existing, s.Paths.SSHManagedFile)
	if integration {
		updated = sshconfig.EnsureInclude(existing, s.Paths.SSHManagedFile)
	}
	report := app.SaveReport{}
	if err := WriteAtomic(s.Paths.ConfigFile, data, 0o600); err != nil {
		report.Files = append(report.Files, app.SaveFile{Path: s.Paths.ConfigFile, Status: "failed", Err: err})
		return report, fmt.Errorf("save %s: %w", s.Paths.ConfigFile, err)
	}
	report.Files = append(report.Files, app.SaveFile{Path: s.Paths.ConfigFile, Status: "saved"})
	if err := WriteAtomic(s.Paths.SSHManagedFile, managed, 0o600); err != nil {
		report.Files = append(report.Files, app.SaveFile{Path: s.Paths.SSHManagedFile, Status: "failed", Err: err})
		return report, fmt.Errorf("save %s: %w", s.Paths.SSHManagedFile, err)
	}
	report.Files = append(report.Files, app.SaveFile{Path: s.Paths.SSHManagedFile, Status: "saved"})
	if err := WriteAtomic(s.Paths.SSHConfigFile, updated, fileMode(s.Paths.SSHConfigFile, 0o600)); err != nil {
		report.Files = append(report.Files, app.SaveFile{Path: s.Paths.SSHConfigFile, Status: "failed", Err: err})
		return report, fmt.Errorf("save %s: %w", s.Paths.SSHConfigFile, err)
	}
	report.Files = append(report.Files, app.SaveFile{Path: s.Paths.SSHConfigFile, Status: "saved"})
	return report, nil
}

func toSnapshot(settings Settings, path string) app.SettingsSnapshot {
	snapshot := app.SettingsSnapshot{ConfigPath: path, ConfigDir: filepath.Dir(path), Profiles: map[string]app.ProfileSettings{}}
	for name, profile := range settings.Profiles {
		p := app.ProfileSettings{Integration: profile.SSH.Integration, StoredIdentityFile: profile.SSH.IdentityFile, SSH: app.SSHOptions{User: profile.SSH.User, Port: profile.SSH.Port}}
		if profile.Regions != nil {
			regions := append([]string(nil), (*profile.Regions)...)
			p.Regions = &regions
		}
		if profile.SSH.IdentityFile != "" {
			p.SSH.IdentityFile = profile.SSH.IdentityFile
			if value, err := ExpandSavedPath(profile.SSH.IdentityFile, filepath.Dir(path)); err == nil {
				p.SSH.IdentityFile = value
			}
		}
		snapshot.Profiles[name] = p
	}
	return snapshot
}

func fromSnapshot(snapshot app.SettingsSnapshot) Settings {
	settings := Settings{Profiles: map[string]Profile{}}
	for name, profile := range snapshot.Profiles {
		identity := profile.StoredIdentityFile
		if identity == "" {
			identity = profile.SSH.IdentityFile
		}
		p := Profile{SSH: SSH{User: profile.SSH.User, IdentityFile: identity, Port: profile.SSH.Port, Integration: profile.Integration}}
		if profile.Regions != nil {
			regions := append([]string(nil), (*profile.Regions)...)
			p.Regions = &regions
		}
		settings.Profiles[name] = p
	}
	return settings
}

func readFileState(path string) (app.FileState, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return app.FileState{Path: path}, nil
	}
	if err != nil {
		return app.FileState{}, err
	}
	resolved := path
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err = filepath.EvalSymlinks(path)
		if err != nil {
			return app.FileState{}, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return app.FileState{}, err
	}
	return app.FileState{Path: path, Exists: true, Data: data, Mode: uint32(info.Mode().Perm()), Resolved: resolved}, nil
}

func sameFileState(a, b app.FileState) bool {
	if a.Exists != b.Exists || a.Mode != b.Mode || a.Resolved != b.Resolved {
		return false
	}
	return string(a.Data) == string(b.Data)
}
func fileMode(path string, fallback os.FileMode) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return fallback
}
