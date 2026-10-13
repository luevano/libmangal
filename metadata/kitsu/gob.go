package kitsu

import "encoding/gob"

// gob stores need the concrete types of the interfaces they hold (same names as mangal's).
func init() {
	gob.RegisterName("kitsu-manga", &Manga{})
	gob.RegisterName("kitsu-user", &User{})
	gob.RegisterName("kitsu-token", &Token{})
}
