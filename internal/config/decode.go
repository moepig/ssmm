package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/uncho/ssmm/internal/target"
)

type Settings struct {
	Profiles map[string]Profile `toml:"profiles"`
}

type Profile struct {
	AWSProfile string    `toml:"aws_profile,omitempty"`
	Regions    *[]string `toml:"regions"`
	SSH        SSH       `toml:"ssh,omitempty"`
}

type SSH struct {
	User         string `toml:"user"`
	IdentityFile string `toml:"identity_file"`
	Port         int    `toml:"port"`
	Integration  bool   `toml:"integration"`
}

func Decode(data []byte, configDir string) (Settings, error) {
	return decode(data, configDir, true)
}

// DecodeLenient keeps the TOML/type/unknown-key checks while deferring
// profile-specific semantic checks until the caller has applied overrides.
func DecodeLenient(data []byte, configDir string) (Settings, error) {
	return decode(data, configDir, false)
}

func decode(data []byte, configDir string, validate bool) (Settings, error) {
	var settings Settings
	meta, err := toml.Decode(string(data), &settings)
	if err != nil {
		return Settings{}, fmt.Errorf("decode config: %w", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) != 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		return Settings{}, fmt.Errorf("unknown config key(s): %s", strings.Join(keys, ", "))
	}
	if settings.Profiles == nil {
		settings.Profiles = map[string]Profile{}
	}
	for name := range settings.Profiles {
		if err := ValidateProfileName(name); err != nil {
			return Settings{}, err
		}
	}
	if validate {
		if err := settings.Validate(configDir); err != nil {
			return Settings{}, err
		}
	}
	if settings.Profiles == nil {
		settings.Profiles = map[string]Profile{}
	}
	return settings, nil
}

func ReadLenient(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Settings{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return DecodeLenient(data, filepath.Dir(path))
}

func (s Settings) ValidateProfile(name, configDir string) error {
	profile, ok := s.Profiles[name]
	if !ok {
		return nil
	}
	if profile.AWSProfile != "" {
		if err := target.ValidateProfileName(profile.AWSProfile); err != nil {
			return fmt.Errorf("profiles.%s.aws_profile: %w", name, err)
		}
	}
	if profile.Regions != nil {
		if len(*profile.Regions) == 0 {
			return fmt.Errorf("profiles.%s.regions must not be empty", name)
		}
		for _, region := range *profile.Regions {
			if !IsCommercialRegion(region) {
				return fmt.Errorf("profiles.%s.regions contains invalid region %q", name, region)
			}
		}
	}
	if profile.SSH.User != "" && strings.TrimSpace(profile.SSH.User) == "" {
		return fmt.Errorf("profiles.%s.ssh.user is empty", name)
	}
	if profile.SSH.Port != 0 && (profile.SSH.Port < 1 || profile.SSH.Port > 65535) {
		return fmt.Errorf("profiles.%s.ssh.port is out of range", name)
	}
	if strings.Contains(profile.SSH.IdentityFile, "${") {
		return fmt.Errorf("profiles.%s.ssh.identity_file contains unsupported ${...}", name)
	}
	if profile.SSH.IdentityFile != "" {
		if _, err := ExpandSavedPath(profile.SSH.IdentityFile, configDir); err != nil {
			return fmt.Errorf("profiles.%s.ssh.identity_file: %w", name, err)
		}
	}
	return nil
}

func Read(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Settings{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return Decode(data, filepath.Dir(path))
}

func (s Settings) Validate(configDir string) error {
	for name, profile := range s.Profiles {
		if err := ValidateProfileName(name); err != nil {
			return err
		}
		if profile.AWSProfile != "" {
			if err := target.ValidateProfileName(profile.AWSProfile); err != nil {
				return fmt.Errorf("profiles.%s.aws_profile: %w", name, err)
			}
		}
		if profile.Regions != nil {
			if len(*profile.Regions) == 0 {
				return fmt.Errorf("profiles.%s.regions must not be empty", name)
			}
			seen := map[string]struct{}{}
			for _, region := range *profile.Regions {
				if !IsCommercialRegion(region) {
					return fmt.Errorf("profiles.%s.regions contains invalid region %q", name, region)
				}
				if _, ok := seen[region]; ok {
					continue
				}
				seen[region] = struct{}{}
			}
		}
		if profile.SSH.User != "" && strings.TrimSpace(profile.SSH.User) == "" {
			return fmt.Errorf("profiles.%s.ssh.user is empty", name)
		}
		if profile.SSH.Port != 0 && (profile.SSH.Port < 1 || profile.SSH.Port > 65535) {
			return fmt.Errorf("profiles.%s.ssh.port is out of range", name)
		}
		if strings.Contains(profile.SSH.IdentityFile, "${") {
			return fmt.Errorf("profiles.%s.ssh.identity_file contains unsupported ${...}", name)
		}
		if profile.SSH.IdentityFile != "" {
			if _, err := ExpandSavedPath(profile.SSH.IdentityFile, configDir); err != nil {
				return fmt.Errorf("profiles.%s.ssh.identity_file: %w", name, err)
			}
		}
	}
	return nil
}

func (s Settings) Profile(name string) Profile {
	if p, ok := s.Profiles[name]; ok {
		return p
	}
	return Profile{}
}

func (s Settings) EffectiveRegions(name, override string) (*[]string, string, error) {
	if override != "" {
		if !IsCommercialRegion(override) {
			return nil, "", fmt.Errorf("invalid region %q", override)
		}
		regions := []string{override}
		return &regions, "--region", nil
	}
	p := s.Profile(name)
	if p.Regions == nil {
		return nil, "all", nil
	}
	regions := unique(*p.Regions)
	return &regions, "profiles." + name + ".regions", nil
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

var regionRE = regexp.MustCompile(`^[a-z]{2}(?:-gov)?-[a-z]+-\d+$`)

func IsCommercialRegion(region string) bool {
	return regionRE.MatchString(region) && !strings.HasPrefix(region, "cn-") && !strings.Contains(region, "-gov-")
}

func ValidateProfileName(name string) error {
	if name == "" || strings.ContainsAny(name, "\x00\r\n /\\") {
		return fmt.Errorf("invalid profile name %q", name)
	}
	return nil
}

func ExpandSavedPath(value, configDir string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("path contains a control character")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if value == "~" {
		return home, nil
	}
	if strings.HasPrefix(value, "~/") {
		value = filepath.Join(home, value[2:])
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(configDir, value)
	}
	return filepath.Clean(value), nil
}

func ExpandFlagPath(value, workingDir string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("path contains a control character")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if value == "~" {
		return home, nil
	}
	if strings.HasPrefix(value, "~/") {
		value = filepath.Join(home, value[2:])
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(workingDir, value)
	}
	return filepath.Clean(value), nil
}

func IsPrivateIPv4(value string) bool { ip := net.ParseIP(value); return ip != nil && ip.To4() != nil }
