package libmangal

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// SettingKind is the type of a ProviderSetting's value. Values are always text, as in a
// config file.
type SettingKind string

const (
	// SettingBool is "true" or "false" (strconv.ParseBool).
	SettingBool SettingKind = "bool"
	// SettingString is any text.
	SettingString SettingKind = "string"
	// SettingInt is a whole number.
	SettingInt SettingKind = "int"
	// SettingDuration is a Go duration, e.g. "500ms".
	SettingDuration SettingKind = "duration"
	// SettingEnum is one of the setting's Values.
	SettingEnum SettingKind = "enum"
	// SettingList is comma-separated items, see SplitList.
	SettingList SettingKind = "list"
	// SettingPath is a file or directory path.
	SettingPath SettingKind = "path"
	// SettingSecret is text the app keeps with its secrets and never shows.
	SettingSecret SettingKind = "secret"
)

var settingKinds = []SettingKind{
	SettingBool, SettingString, SettingInt, SettingDuration,
	SettingEnum, SettingList, SettingPath, SettingSecret,
}

// keys end up in config keys and environment variable names
var settingKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ProviderSetting is a setting the user may change for a provider. The provider declares
// it, the app stores, shows and validates it, and hands the values back (SettingsFunc).
type ProviderSetting struct {
	// Key of the setting, unique in the provider: lowercase letters, digits and "_",
	// starting with a letter. E.g. "data_saver".
	Key string `json:"key"`

	// Name shown to the user. May be empty, then Key is shown.
	Name string `json:"name,omitempty"`

	// Description of what the setting does. May be empty.
	Description string `json:"description,omitempty"`

	// Kind of the value.
	Kind SettingKind `json:"kind"`

	// Default value, as text. Must be valid for Kind (empty is valid for every kind but
	// bool, int, duration and enum).
	Default string `json:"default,omitempty"`

	// Values are the choices of a SettingEnum, non-empty for it. Unused by other kinds.
	Values []string `json:"values,omitempty"`

	// Warning shown when the user changes the setting, e.g. a risk to the account. May be
	// empty.
	Warning string `json:"warning,omitempty"`
}

// Check reports whether value is valid for the setting's Kind.
func (s ProviderSetting) Check(value string) error {
	var err error
	switch s.Kind {
	case SettingBool:
		_, err = strconv.ParseBool(value)
	case SettingInt:
		_, err = strconv.ParseInt(value, 10, 64)
	case SettingDuration:
		_, err = time.ParseDuration(value)
	case SettingEnum:
		if !slices.Contains(s.Values, value) {
			err = fmt.Errorf("not one of %s", strings.Join(s.Values, ", "))
		}
	case SettingString, SettingList, SettingPath, SettingSecret:
	default:
		return fmt.Errorf("setting %q: unknown kind %q", s.Key, s.Kind)
	}
	if err != nil {
		return fmt.Errorf("setting %q: invalid %s %q: %w", s.Key, s.Kind, value, err)
	}
	return nil
}

// Validate checks the setting's Key, Kind, Values and Default.
func (s ProviderSetting) Validate() error {
	if !settingKeyRe.MatchString(s.Key) {
		return fmt.Errorf("invalid setting key %q", s.Key)
	}
	if !slices.Contains(settingKinds, s.Kind) {
		return fmt.Errorf("setting %q: unknown kind %q", s.Key, s.Kind)
	}
	if s.Kind == SettingEnum && len(s.Values) == 0 {
		return fmt.Errorf("setting %q: enum without values", s.Key)
	}
	if err := s.Check(s.Default); err != nil {
		return fmt.Errorf("default: %w", err)
	}
	return nil
}

// ProviderTraits tell the app how to treat the provider. All false by default.
type ProviderTraits struct {
	// PartialChapters: the provider lists only some of a manga's chapters (e.g. the first
	// and latest), so a library having more isn't a sign of a wrong match.
	PartialChapters bool `json:"partial_chapters,omitempty"`

	// AvoidUnattended: background jobs (maintenance checks) shouldn't make requests to it
	// unless the user turns them on, e.g. because of an account lockout risk. Watches the
	// user set up are not unattended.
	AvoidUnattended bool `json:"avoid_unattended,omitempty"`
}

// SettingsFunc returns the values the user set for a provider's settings, by key; keys
// not in it get their defaults, nil means all defaults. Providers' loader options take
// one and call it when the provider loads; the app decides where values come from.
type SettingsFunc func(providerID string) map[string]string

// Setting returns the provider's setting with the given key.
func (p ProviderInfo) Setting(key string) (ProviderSetting, bool) {
	for _, s := range p.Settings {
		if s.Key == key {
			return s, true
		}
	}
	return ProviderSetting{}, false
}

// SettingValues returns a value for every declared setting: the one in set if there is
// one, else its Default. Values in set are checked; keys in set the provider doesn't
// declare are ignored (they may belong to a setting it no longer has).
func (p ProviderInfo) SettingValues(set map[string]string) (map[string]string, error) {
	values := make(map[string]string, len(p.Settings))
	var errs []error
	for _, s := range p.Settings {
		value, ok := set[s.Key]
		if !ok {
			values[s.Key] = s.Default
			continue
		}
		if err := s.Check(value); err != nil {
			errs = append(errs, err)
			continue
		}
		values[s.Key] = value
	}
	if len(errs) != 0 {
		return nil, fmt.Errorf("provider %q: %w", p.ID, errors.Join(errs...))
	}
	return values, nil
}

// SplitList splits a SettingList value into its trimmed, non-empty items.
func SplitList(value string) []string {
	items := []string{}
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
