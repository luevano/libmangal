package kitsu

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/luevano/libmangal/metadata"
)

const profileURL = "https://kitsu.app/users/"

var _ metadata.User = (*User)(nil)

// User is the user model for Kitsu.
//
// Note that User fields don't match the method names
// to avoid collisions with the interface.
type User struct {
	IDProvider     int    `json:"id"`
	NameProvider   string `json:"name"`
	Slug           string `json:"slug"`
	AboutProvider  string `json:"about"`
	AvatarProvider Image  `json:"avatar"`
}

func newUser(r resource) (*User, error) {
	id, err := strconv.Atoi(r.ID)
	if err != nil {
		return nil, fmt.Errorf("user id %q isn't an integer", r.ID)
	}
	var a struct {
		Name   string `json:"name"`
		Slug   string `json:"slug"`
		About  string `json:"about"`
		Avatar *Image `json:"avatar"`
	}
	if err := json.Unmarshal(r.Attributes, &a); err != nil {
		return nil, fmt.Errorf("user %d: %w", id, err)
	}
	u := &User{IDProvider: id, NameProvider: a.Name, Slug: a.Slug, AboutProvider: a.About}
	if a.Avatar != nil {
		u.AvatarProvider = *a.Avatar
	}
	return u, nil
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
	return u.AvatarProvider.Largest()
}

// URL is the user's URL on the metadata provider website.
func (u *User) URL() string {
	if u.Slug == "" {
		return profileURL + strconv.Itoa(u.IDProvider)
	}
	return profileURL + u.Slug
}

// Source provider of the user.
//
// For example if coming from Anilist: IDSourceAnilist.
func (u *User) Source() metadata.IDSource {
	return metadata.IDSourceKitsu
}
