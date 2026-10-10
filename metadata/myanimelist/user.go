package myanimelist

import (
	"strconv"

	"github.com/luevano/libmangal/metadata"
)

const profileURL = "https://myanimelist.net/profile/"

var _ metadata.User = (*User)(nil)

// TODO: add AnimeStatistics
// TODO: change field types from string to date/date-time
//
// User is the user model for MyAnimeList.
//
// Note that User fields don't match the incoming json
// fields to avoid collisions with the interface.
type User struct {
	IDProvider   int    `json:"id"`
	NameProvider string `json:"name"`
	Picture      string `json:"picture"`
	Gender       string `json:"gender"`
	Birthday     string `json:"birthday"`
	Location     string `json:"location"`
	JoinedAt     string `json:"joined_at"`
	Timezone     string `json:"time_zone" jsonschema:"description=For example 'America/Los_Angeles'"`
	IsSupporter  bool   `json:"is_supporter"`
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
	return ""
}

// Avatar is the URL of the avatar image.
func (u *User) Avatar() string {
	return u.Picture
}

// URL is the user's URL on the metadata provider website.
func (u *User) URL() string {
	return profileURL + u.NameProvider
}

// Source provider of the user.
//
// For example if coming from Anilist: IDSourceAnilist.
func (u *User) Source() metadata.IDSource {
	return metadata.IDSourceMyAnimeList
}
