package mangadata

import (
	"fmt"

	"github.com/luevano/libmangal/metadata"
)

// ChapterInfo is the general information for the chapter.
type ChapterInfo struct {
	// Title is the title of chapter.
	Title string `json:"title"`

	// URL is the url leading to chapter web page.
	URL string `json:"url"`

	// Number of the chapter.
	//
	// Float type allows for extra chapters that usually have
	// numbering like the following: 10.5, 101.1, etc..
	Number float32 `json:"number"`

	// Date is the chapter publication date.
	Date metadata.Date `json:"date"`

	// ScanlationGroup is the group that did the scan/translation.
	//
	// If not an official publication, most of the chapters will belong
	// to a scanlation group.
	ScanlationGroup string `json:"scanlation_group"`

	// Unavailable is the reason the chapter can't be downloaded (e.g. "0 pages", "external"),
	// empty if it can.
	//
	// Providers can still list unavailable chapters so they're shown, but they won't be downloaded.
	Unavailable string `json:"unavailable,omitempty"`
}

// Chapter is what Volume consists of. Each chapter is about 24–40 pages.
type Chapter interface {
	fmt.Stringer

	Info() ChapterInfo

	// Volume gets the Volume that this Chapter is relevant to.
	//
	// Implementation should not make any external requests
	// nor be computationally heavy.
	Volume() Volume
}

// WrappedChapter is a Chapter standing in for another one of the provider's (e.g. with
// another title, volume or manga): the client asks the provider for the pages of the
// chapter Unwrap returns, everything else (names, metadata, ComicInfo.xml) is the
// wrapper's.
type WrappedChapter interface {
	Chapter

	// Unwrap is the provider's own chapter.
	Unwrap() Chapter
}

// ChapterWithComicInfoXML is a Chapter with an already associated ComicInfoXML.
//
// The associated ComicInfoXML will be used instead of the one generated from the metadata.
type ChapterWithComicInfoXML interface {
	Chapter

	// ComicInfoXML will be used to write ComicInfo.xml file.
	//
	// Implementation should not make any external requests.
	// If found is false then the one generated from the metadata is used.
	ComicInfoXML() (comicInfoXML metadata.ComicInfoXML, found bool, err error)
}
