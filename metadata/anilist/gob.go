package anilist

import "encoding/gob"

// gob stores need the concrete types of the interfaces they hold (same names as mangal's).
func init() {
	gob.RegisterName("anilist-manga", &Manga{})
	gob.RegisterName("anilist-user", &User{})
}
