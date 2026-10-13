package myanimelist

import "encoding/gob"

// gob stores need the concrete types of the interfaces they hold (same names as mangal's).
func init() {
	gob.RegisterName("myanimelist-manga", &Manga{})
	gob.RegisterName("myanimelist-user", &User{})
}
