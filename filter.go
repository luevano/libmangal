package libmangal

import "fmt"

// FilterOptions are the user's filters, providers should respect them when the source supports it.
type FilterOptions struct {
	// NSFW if the manga results can contain NSFW content.
	//
	// Only if the source allows to filter by NSFW.
	NSFW bool

	// Language of the manga and chapter results, e.g. "en", empty for any language.
	//
	// Only if the source allows to filter by language.
	Language string

	// AvoidDuplicateChapters keeps only one chapter per number.
	AvoidDuplicateChapters bool

	// ShowUnavailableChapters also shows chapters that can't be downloaded,
	// such as ones hosted on external sites (MangaDex chapters on MangaPlus).
	ShowUnavailableChapters bool
}

// DefaultFilterOptions constructs FilterOptions that filter nothing:
// NSFW included, any language, duplicate and unavailable chapters kept.
func DefaultFilterOptions() FilterOptions {
	return FilterOptions{
		NSFW:                    true,
		Language:                "",
		AvoidDuplicateChapters:  false,
		ShowUnavailableChapters: true,
	}
}

// String is a stable representation of the filters, e.g. for cache keys.
func (f *FilterOptions) String() string {
	return fmt.Sprintf(
		"filters[%t&%s&%t&%t]",
		f.NSFW,
		f.Language,
		f.AvoidDuplicateChapters,
		f.ShowUnavailableChapters,
	)
}
