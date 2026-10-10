package anilist

import (
	"strconv"

	"github.com/luevano/libmangal/metadata"
)

var _ metadata.User = (*User)(nil)

// User is the user model for Anilist.
//
// Note that User fields don't match the incoming json
// fields to avoid collisions with the interface.
type User struct {
	IDProvider     int    `json:"id" jsonschema:"description=The ID of the user."`
	NameProvider   string `json:"name" jsonschema:"description=The name of the user."`
	AboutProvider  string `json:"about" jsonschema:"description=The bio written by user (Markdown)."`
	AvatarProvider struct {
		Large  string `json:"large" jsonschema:"description=The avatar of user at its largest size."`
		Medium string `json:"medium" jsonschema:"description=The avatar of user at medium size."`
	} `json:"avatar" jsonschema:"description=The user's avatar images."`
	BannerImage string `json:"bannerImage" jsonschema:"description=The user's banner images."`
	Options     struct {
		TitleLanguage       string `json:"titleLanguage" jsonschema:"enum=ROMAJI,enum=ENGLISH,enum=NATIVE,enum=ROMAJI_STYLISED,enum=ENGLISH_STYLISED,enum=NATIVE_STYLISED"`
		DisplayAdultContent bool   `json:"displayAdultContent" jsonschema:"description=Whether the user has enabled viewing of 18+ content."`
		ProfileColor        string `json:"profileColor" jsonschema:"description=Profile highlight color (blue, purple, pink, orange, red, green, gray)."`
		Timezone            string `json:"timezone" jsonschema:"description=The user's timezone offset (Auth user only)."`
	} `json:"options" jsonschema:"description=The user's general options."`
	SiteURL       string `json:"siteUrl" jsonschema:"description=The URL for the user page on the AniList website."`
	CreatedAt     int    `json:"createdAt" jsonschema:"description=When the user's account was created. (Does not exist for accounts created before 2020)."`
	UpdatedAt     int    `json:"updatedAt" jsonschema:"description=When the user's data was last updated."`
	PreviousNames []struct {
		Name      string `json:"name" jsonschema:"description=The name of the user."`
		CreatedAt int    `json:"createdAt" jsonschema:"description=When the user first changed from this name."`
		UpdatedAt int    `json:"updatedAt" jsonschema:"description=When the user most recently changed from this name."`
	} `json:"previous_names" jsonschema:"description=The user's previously used names."`
}

// String is the short representation of the user.
// Must be non-empty.
//
// For example "`Name` (`ID`)".
func (u *User) String() string {
	return u.Name() + " (" + strconv.Itoa(u.ID()) + ")"
}

// ID is the id of the user.
func (u *User) ID() int {
	return u.IDProvider
}

// Name of the user.
func (u *User) Name() string {
	return u.NameProvider
}

// About is the about section of the user.
func (u *User) About() string {
	return u.AboutProvider
}

// Avatar is the URL of the avatar image.
func (u *User) Avatar() string {
	avatar := u.AvatarProvider.Large
	if avatar == "" {
		avatar = u.AvatarProvider.Medium
	}
	return avatar
}

// URL is the user's URL on the metadata provider website.
func (u *User) URL() string {
	return u.SiteURL
}

// Source provider of the user.
//
// For example if coming from Anilist: IDSourceAnilist.
func (u *User) Source() metadata.IDSource {
	return metadata.IDSourceAnilist
}
