package libmangal

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func settingsInfo(settings ...ProviderSetting) ProviderInfo {
	return ProviderInfo{ID: "x", Name: "X", Version: "0.1.0", Settings: settings}
}

func TestProviderInfoValidateSettings(t *testing.T) {
	good := settingsInfo(
		ProviderSetting{Key: "data_saver", Kind: SettingBool, Default: "false"},
		ProviderSetting{Key: "quality", Kind: SettingEnum, Default: "high", Values: []string{"low", "high"}},
		ProviderSetting{Key: "interval", Kind: SettingDuration, Default: "500ms"},
		ProviderSetting{Key: "groups", Kind: SettingList},
		ProviderSetting{Key: "secret", Kind: SettingSecret},
	)
	if err := good.Validate(); err != nil {
		t.Fatalf("valid settings: %v", err)
	}
	for name, bad := range map[string]ProviderSetting{
		"empty key":       {Kind: SettingString},
		"dotted key":      {Key: "a.b", Kind: SettingString},
		"hyphen key":      {Key: "a-b", Kind: SettingString},
		"upper key":       {Key: "Quality", Kind: SettingString},
		"unknown kind":    {Key: "a", Kind: "float"},
		"enum no values":  {Key: "a", Kind: SettingEnum},
		"enum bad def":    {Key: "a", Kind: SettingEnum, Default: "mid", Values: []string{"low"}},
		"bool no default": {Key: "a", Kind: SettingBool},
		"bad int":         {Key: "a", Kind: SettingInt, Default: "1.5"},
		"bad duration":    {Key: "a", Kind: SettingDuration, Default: "5"},
	} {
		if err := settingsInfo(bad).Validate(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	dup := ProviderSetting{Key: "a", Kind: SettingString}
	if err := settingsInfo(dup, dup).Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate key: %v", err)
	}
}

func TestProviderInfoSettingValues(t *testing.T) {
	info := settingsInfo(
		ProviderSetting{Key: "data_saver", Kind: SettingBool, Default: "false"},
		ProviderSetting{Key: "quality", Kind: SettingEnum, Default: "high", Values: []string{"low", "high"}},
	)
	values, err := info.SettingValues(map[string]string{"quality": "low", "gone": "x"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"data_saver": "false", "quality": "low"}
	if len(values) != len(want) || values["data_saver"] != "false" || values["quality"] != "low" {
		t.Errorf("values = %v, want %v", values, want)
	}
	if values, err := info.SettingValues(nil); err != nil || values["quality"] != "high" {
		t.Errorf("nil set: %v, %v", values, err)
	}
	_, err = info.SettingValues(map[string]string{"quality": "mid", "data_saver": "maybe"})
	if err == nil || !strings.Contains(err.Error(), `"quality"`) || !strings.Contains(err.Error(), `"data_saver"`) {
		t.Errorf("invalid values: %v", err)
	}
	if s, ok := info.Setting("quality"); !ok || s.Default != "high" {
		t.Errorf("Setting(quality) = %v, %v", s, ok)
	}
	if _, ok := info.Setting("nope"); ok {
		t.Error("Setting(nope) found")
	}
}

func TestSplitList(t *testing.T) {
	if got := SplitList(" a, b ,,c "); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("SplitList = %q", got)
	}
	if got := SplitList(""); got == nil || len(got) != 0 {
		t.Errorf("SplitList(\"\") = %#v", got)
	}
}

func TestProviderInfoSettingsJSON(t *testing.T) {
	plain, err := json.Marshal(ProviderInfo{ID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "settings") || strings.Contains(string(plain), "traits") {
		t.Errorf("empty settings/traits are written: %s", plain)
	}
	info := settingsInfo(ProviderSetting{Key: "a", Kind: SettingString})
	info.Traits = ProviderTraits{PartialChapters: true}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var back ProviderInfo
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Traits.PartialChapters || len(back.Settings) != 1 || back.Settings[0].Kind != SettingString {
		t.Errorf("round trip: %s -> %+v", data, back)
	}
}
