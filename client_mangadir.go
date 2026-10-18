package libmangal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sync"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/spf13/afero"
)

// FilenameMangaDirInfo is the file with the MangaDirInfo written in every manga directory.
const FilenameMangaDirInfo = ".libmangal.json"

// MangaDirInfo identifies the manga downloaded in a directory, so the directory is
// found again when the name it would get changes (title updates, renamed providers,
// name template changes). See DownloadOptions.ReuseMangaDir.
type MangaDirInfo struct {
	// Provider is the ID of the provider the manga was last downloaded from.
	Provider string `json:"provider"`

	// Manga is the provider's manga.
	Manga mangadata.MangaInfo `json:"manga"`

	// MetadataIDs are the ids of the manga's metadata, other than the provider's own
	// (e.g. its Anilist id).
	MetadataIDs []metadata.ID `json:"metadata_ids,omitempty"`
}

func newMangaDirInfo(provider ProviderInfo, manga mangadata.Manga) MangaDirInfo {
	info := MangaDirInfo{Provider: provider.ID, Manga: manga.Info()}
	meta := manga.Metadata()
	if metadata.Validate(meta) != nil {
		return info
	}
	for _, id := range append([]metadata.ID{meta.ID()}, meta.ExtraIDs()...) {
		if id.Source != metadata.IDSourceProvider && id.Raw != "" {
			info.MetadataIDs = append(info.MetadataIDs, id)
		}
	}
	return info
}

// MangaDirIndex is an index of the downloaded mangas' directories kept by the application
// (e.g. in a database), asked before scanning the MangaDirInfo files. See ClientOptions.MangaDirIndex.
type MangaDirIndex interface {
	// FindMangaDir returns the name of the directory under parent with the manga,
	// matching like MatchMangaDir.
	FindMangaDir(parent string, provider ProviderInfo, info MangaDirInfo) (name string, found bool, err error)

	// RememberMangaDir records the manga of the directory parent/name.
	RememberMangaDir(parent, name string, info MangaDirInfo) error
}

// ChapterIndex knows the chapters downloaded in each manga directory. See ClientOptions.ChapterIndex.
type ChapterIndex interface {
	// FindChapter returns the path of the chapter with the same number in the format,
	// downloaded in mangaDir (by any provider), and the provider id it came from.
	FindChapter(mangaDir string, chapter mangadata.Chapter, format Format) (path, provider string, found bool, err error)
}

// MangaDirIndexContext is a MangaDirIndex whose lookups take a context (e.g. a database
// query that can be cancelled). The client calls these instead when the
// ClientOptions.MangaDirIndex implements it; the plain methods should call them with
// context.Background().
type MangaDirIndexContext interface {
	MangaDirIndex
	FindMangaDirContext(ctx context.Context, parent string, provider ProviderInfo, info MangaDirInfo) (name string, found bool, err error)
	RememberMangaDirContext(ctx context.Context, parent, name string, info MangaDirInfo) error
}

// ChapterIndexContext is a ChapterIndex whose lookup takes a context, called instead of
// FindChapter when the ClientOptions.ChapterIndex implements it.
type ChapterIndexContext interface {
	ChapterIndex
	FindChapterContext(ctx context.Context, mangaDir string, chapter mangadata.Chapter, format Format) (path, provider string, found bool, err error)
}

func findMangaDirIn(ctx context.Context, index MangaDirIndex, parent string, provider ProviderInfo, info MangaDirInfo) (string, bool, error) {
	if index, ok := index.(MangaDirIndexContext); ok {
		return index.FindMangaDirContext(ctx, parent, provider, info)
	}
	return index.FindMangaDir(parent, provider, info)
}

func rememberMangaDirIn(ctx context.Context, index MangaDirIndex, parent, name string, info MangaDirInfo) error {
	if index, ok := index.(MangaDirIndexContext); ok {
		return index.RememberMangaDirContext(ctx, parent, name, info)
	}
	return index.RememberMangaDir(parent, name, info)
}

func findChapterIn(ctx context.Context, index ChapterIndex, mangaDir string, chapter mangadata.Chapter, format Format) (string, string, bool, error) {
	if index, ok := index.(ChapterIndexContext); ok {
		return index.FindChapterContext(ctx, mangaDir, chapter, format)
	}
	return index.FindChapter(mangaDir, chapter, format)
}

// Manga directory matches, from MatchMangaDir.
const (
	MangaDirNoMatch = iota
	// MangaDirMetadataMatch shares a metadata id (e.g. the same Anilist manga from another provider).
	MangaDirMetadataMatch
	// MangaDirMangaMatch is the same provider (current or old id) and manga id.
	MangaDirMangaMatch
)

// MatchMangaDir says how well the info of a directory (have) matches the wanted manga,
// downloaded with the provider. Higher is better.
func MatchMangaDir(provider ProviderInfo, want, have MangaDirInfo) int {
	if provider.Matches(have.Provider) && have.Manga.ID == want.Manga.ID {
		return MangaDirMangaMatch
	}
	for _, id := range want.MetadataIDs {
		if slices.ContainsFunc(have.MetadataIDs, func(other metadata.ID) bool {
			return other.Source == id.Source && other.Raw == id.Raw
		}) {
			return MangaDirMetadataMatch
		}
	}
	return MangaDirNoMatch
}

// ReadMangaDirInfo reads the MangaDirInfo of a manga directory.
func ReadMangaDirInfo(fsys afero.Fs, dir string) (MangaDirInfo, error) {
	var info MangaDirInfo
	data, err := afero.ReadFile(fsys, filepath.Join(dir, FilenameMangaDirInfo))
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("%s: %w", filepath.Join(dir, FilenameMangaDirInfo), err)
	}
	return info, nil
}

// mangaDirIndex caches the MangaDirInfo of the directories under each parent directory,
// scanned once per client.
type mangaDirIndex struct {
	mu   sync.Mutex
	dirs map[string][]indexedDir
}

type indexedDir struct {
	name string
	info MangaDirInfo
}

func (idx *mangaDirIndex) load(fsys afero.Fs, parent string) ([]indexedDir, error) {
	if dirs, ok := idx.dirs[parent]; ok {
		return dirs, nil
	}
	entries, err := afero.ReadDir(fsys, parent)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var dirs []indexedDir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := ReadMangaDirInfo(fsys, filepath.Join(parent, entry.Name()))
		if err != nil {
			// not a manga directory or not one with info yet
			continue
		}
		dirs = append(dirs, indexedDir{name: entry.Name(), info: info})
	}
	if idx.dirs == nil {
		idx.dirs = map[string][]indexedDir{}
	}
	idx.dirs[parent] = dirs
	return dirs, nil
}

// find returns the directory under parent with the same manga (same provider, current
// or old id, and manga id), or else one sharing a metadata id.
func (idx *mangaDirIndex) find(fsys afero.Fs, parent string, provider ProviderInfo, info MangaDirInfo) (string, bool, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	dirs, err := idx.load(fsys, parent)
	if err != nil {
		return "", false, err
	}
	for {
		best, name := MangaDirNoMatch, ""
		for _, d := range dirs {
			if match := MatchMangaDir(provider, info, d.info); match > best {
				best, name = match, d.name
			}
		}
		if best == MangaDirNoMatch {
			return "", false, nil
		}
		// the directory may have been moved or deleted since the scan
		exists, err := afero.DirExists(fsys, filepath.Join(parent, name))
		if err != nil {
			return "", false, err
		}
		if exists {
			return name, true, nil
		}
		dirs = slices.DeleteFunc(dirs, func(d indexedDir) bool { return d.name == name })
		idx.dirs[parent] = dirs
	}
}

func (idx *mangaDirIndex) remember(parent, name string, info MangaDirInfo) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	dirs, ok := idx.dirs[parent]
	if !ok {
		// not scanned yet, the scan will find it
		return
	}
	dirs = slices.DeleteFunc(dirs, func(d indexedDir) bool { return d.name == name })
	idx.dirs[parent] = append(dirs, indexedDir{name: name, info: info})
}

// chapterDirs are the directories a chapter is downloaded to.
type chapterDirs struct {
	// chapter is where the chapter goes.
	chapter string
	// parent is where manga directories go, manga is the manga directory ("" if none).
	parent, manga string
	// meta is where series.json, the cover and the banner go.
	meta string
}

// ChapterDir returns the directory the chapter is (or would be) downloaded to with the options,
// including a reused manga directory (DownloadOptions.ReuseMangaDir).
func (c *Client) ChapterDir(chapter mangadata.Chapter, options DownloadOptions) (string, error) {
	return c.ChapterDirContext(context.Background(), chapter, options)
}

// ChapterDirContext is ChapterDir with a context, passed to the ClientOptions.MangaDirIndex.
func (c *Client) ChapterDirContext(ctx context.Context, chapter mangadata.Chapter, options DownloadOptions) (string, error) {
	dirs, err := c.chapterDirs(ctx, chapter, options)
	return dirs.chapter, err
}

func (c *Client) chapterDirs(ctx context.Context, chapter mangadata.Chapter, options DownloadOptions) (chapterDirs, error) {
	dirs := chapterDirs{parent: options.Directory, meta: options.Directory}
	if options.CreateProviderDir {
		name, err := c.providerDirName(options)
		if err != nil {
			return dirs, err
		}
		dirs.parent = filepath.Join(dirs.parent, name)
	}
	dirs.chapter = dirs.parent

	if options.CreateMangaDir {
		manga := chapter.Volume().Manga()
		name := c.MangaName(manga)
		if options.ReuseMangaDir {
			found, ok, err := c.findMangaDir(ctx, dirs.parent, newMangaDirInfo(c.Info(), manga))
			if err != nil {
				return dirs, fmt.Errorf("find the directory of manga %q: %w", manga, err)
			}
			if ok && found != name {
				c.logger.Log("reusing directory %q for manga %q (would be %q)", found, manga, name)
				name = found
			}
		}
		dirs.manga = filepath.Join(dirs.parent, name)
		dirs.chapter = dirs.manga
		dirs.meta = dirs.manga
	}

	if options.CreateVolumeDir {
		dirs.chapter = filepath.Join(dirs.chapter, c.VolumeName(chapter.Volume()))
	}
	return dirs, nil
}

// providerDirName is the provider directory name, or the name it had before
// (ProviderInfo.History) if only that directory exists and ReuseMangaDir is set.
func (c *Client) providerDirName(options DownloadOptions) (string, error) {
	info := c.Info()
	name := c.ProviderName(info)
	if !options.ReuseMangaDir || len(info.History) == 0 {
		return name, nil
	}
	exists, err := afero.DirExists(c.options.FS, filepath.Join(options.Directory, name))
	if err != nil || exists {
		return name, err
	}
	for _, alias := range info.History {
		old := info
		old.ID, old.Name, old.Website, old.History = alias.ID, alias.Name, alias.Website, nil
		if old.Name == "" {
			old.Name = info.Name
		}
		oldName := c.ProviderName(old)
		exists, err := afero.DirExists(c.options.FS, filepath.Join(options.Directory, oldName))
		if err != nil {
			return name, err
		}
		if exists {
			c.logger.Log("reusing directory %q of the provider's old name for %q", oldName, info.Name)
			return oldName, nil
		}
	}
	return name, nil
}

// findMangaDir asks the index (ClientOptions.MangaDirIndex) first, then scans the
// MangaDirInfo files; what the scan finds is added to the index.
func (c *Client) findMangaDir(ctx context.Context, parent string, info MangaDirInfo) (string, bool, error) {
	index := c.options.MangaDirIndex
	if index != nil {
		name, ok, err := findMangaDirIn(ctx, index, parent, c.Info(), info)
		if err != nil {
			return "", false, fmt.Errorf("index: %w", err)
		}
		if ok {
			// the directory may have been moved or deleted since
			exists, err := afero.DirExists(c.options.FS, filepath.Join(parent, name))
			if err != nil || exists {
				return name, exists, err
			}
		}
	}

	name, ok, err := c.mangaDirs.find(c.options.FS, parent, c.Info(), info)
	if err != nil || !ok || index == nil {
		return name, ok, err
	}
	have, err := ReadMangaDirInfo(c.options.FS, filepath.Join(parent, name))
	if err != nil {
		return name, ok, err
	}
	if err := rememberMangaDirIn(ctx, index, parent, name, have); err != nil {
		return name, ok, fmt.Errorf("index: %w", err)
	}
	return name, ok, nil
}

// rememberMangaDir records a downloaded manga directory in the index and the scan cache.
func (c *Client) rememberMangaDir(ctx context.Context, parent, name string, info MangaDirInfo) error {
	c.mangaDirs.remember(parent, name, info)
	if c.options.MangaDirIndex == nil {
		return nil
	}
	if err := rememberMangaDirIn(ctx, c.options.MangaDirIndex, parent, name, info); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	return nil
}

// WriteMangaDirInfo writes info as the FilenameMangaDirInfo of dir, atomically (a temp
// file renamed over it), so a crash never leaves a truncated one.
func WriteMangaDirInfo(fsys afero.Fs, dir string, info MangaDirInfo, mode fs.FileMode) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", FilenameMangaDirInfo, err)
	}
	tmp, err := afero.TempFile(fsys, dir, FilenameMangaDirInfo+".*.tmp")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = fsys.Chmod(tmp.Name(), mode)
	}
	if err == nil {
		err = fsys.Rename(tmp.Name(), filepath.Join(dir, FilenameMangaDirInfo))
	}
	if err != nil {
		_ = fsys.Remove(tmp.Name())
	}
	return err
}
