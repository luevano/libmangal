package mangaupdates

import "encoding/gob"

// gob stores need the concrete types of the interfaces they hold.
func init() {
	gob.RegisterName("mangaupdates-manga", &Manga{})
}
