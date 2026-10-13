package myanimelist

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/luevano/libmangal/metadata"
)

func TestGobRegistered(t *testing.T) {
	// registering again with the same names (as mangal does) must not panic
	gob.RegisterName("myanimelist-manga", &Manga{})
	gob.RegisterName("myanimelist-user", &User{})

	var buf bytes.Buffer
	var in metadata.Metadata = &Manga{}
	if err := gob.NewEncoder(&buf).Encode(&in); err != nil {
		t.Fatal(err)
	}
	var out metadata.Metadata
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(*Manga); !ok {
		t.Errorf("decoded %T", out)
	}
}
