package libmangal

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/spf13/afero"
)

// failingFs fails reading the files named fail, like a copy interrupted midway.
type failingFs struct {
	afero.Fs
	fail string
}

type failingFile struct{ afero.File }

func (f failingFile) Read([]byte) (int, error) { return 0, errWrite }

func (fs failingFs) Open(name string) (afero.File, error) {
	f, err := fs.Fs.Open(name)
	if err == nil && filepath.Base(name) == fs.fail {
		return failingFile{f}, nil
	}
	return f, err
}

func TestMergeDirectoriesInterrupted(t *testing.T) {
	dst := t.TempDir()
	dstFS := afero.NewOsFs()
	existing := filepath.Join(dst, "manga", "ch1.cbz")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := afero.NewMemMapFs()
	_ = afero.WriteFile(src, "/d/manga/ch1.cbz", []byte("new"), 0o644)

	// overwriting an existing file
	err := mergeDirectories(0o755, 0o644, dstFS, dst, failingFs{src, "ch1.cbz"}, "/d", nil)
	if !errors.Is(err, errWrite) {
		t.Fatalf("got %v, want %v", err, errWrite)
	}
	if b, err := os.ReadFile(existing); err != nil || string(b) != "old" {
		t.Errorf("existing chapter: %q, %v, want it untouched", b, err)
	}

	// a new chapter directory (images format)
	_ = afero.WriteFile(src, "/d/manga/ch2/0001.jpg", []byte("1"), 0o644)
	_ = afero.WriteFile(src, "/d/manga/ch2/0002.jpg", []byte("2"), 0o644)
	err = mergeDirectories(0o755, 0o644, dstFS, dst, failingFs{src, "0002.jpg"}, "/d", nil)
	if !errors.Is(err, errWrite) {
		t.Fatalf("got %v, want %v", err, errWrite)
	}
	entries, _ := os.ReadDir(filepath.Join(dst, "manga"))
	for _, e := range entries {
		if e.Name() != "ch1.cbz" {
			t.Errorf("left %s behind", e.Name())
		}
	}

	// and a successful merge
	if err := mergeDirectories(0o755, 0o644, dstFS, dst, src, "/d", nil); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"manga/ch1.cbz": "new", "manga/ch2/0002.jpg": "2"} {
		if b, err := os.ReadFile(filepath.Join(dst, path)); err != nil || string(b) != want {
			t.Errorf("%s: %q, %v, want %q", path, b, err, want)
		}
	}
}

// syncFs records the files synced before being renamed.
type syncFs struct {
	afero.Fs
	mu      sync.Mutex
	synced  map[string]bool
	renamed []string
}

type syncFile struct {
	afero.File
	fs *syncFs
}

func (f syncFile) Sync() error {
	f.fs.mu.Lock()
	f.fs.synced[f.Name()] = true
	f.fs.mu.Unlock()
	return f.File.Sync()
}

func (fs *syncFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	f, err := fs.Fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return syncFile{f, fs}, nil
}

func (fs *syncFs) Rename(oldname, newname string) error {
	fs.mu.Lock()
	if fs.synced[oldname] {
		fs.renamed = append(fs.renamed, newname)
	}
	fs.mu.Unlock()
	return fs.Fs.Rename(oldname, newname)
}

func TestCopyFileSyncs(t *testing.T) {
	src := afero.NewMemMapFs()
	_ = afero.WriteFile(src, "/d/ch1.cbz", []byte("new"), 0o644)
	dst := &syncFs{Fs: afero.NewMemMapFs(), synced: map[string]bool{}}
	if err := mergeDirectories(0o755, 0o644, dst, "/out", src, "/d", nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dst.renamed, []string{"/out/ch1.cbz"}) {
		t.Errorf("synced and renamed %q, want /out/ch1.cbz", dst.renamed)
	}
}

func TestMergeDirectoriesLeavesOthersPartFiles(t *testing.T) {
	dst := afero.NewMemMapFs()
	// another download writing the same chapter
	other := "/out/manga/ch1.cbz" + partSuffix
	_ = afero.WriteFile(dst, other, []byte("other"), 0o644)
	_ = dst.MkdirAll("/out/manga/ch2"+partSuffix, 0o755)

	src := afero.NewMemMapFs()
	_ = afero.WriteFile(src, "/d/manga/ch1.cbz", []byte("new"), 0o644)
	_ = afero.WriteFile(src, "/d/manga/ch2/0001.jpg", []byte("1"), 0o644)
	if err := mergeDirectories(0o755, 0o644, dst, "/out", src, "/d", nil); err != nil {
		t.Fatal(err)
	}
	if b, err := afero.ReadFile(dst, other); err != nil || string(b) != "other" {
		t.Errorf("%s: %q, %v, want it untouched", other, b, err)
	}
	if ok, _ := afero.Exists(dst, "/out/manga/ch2/0001.jpg"); !ok {
		t.Error("ch2 not written")
	}
}

func TestMergeDirectoriesReplace(t *testing.T) {
	dst := afero.NewMemMapFs()
	_ = afero.WriteFile(dst, "/out/ch1/0001.jpg", []byte("old"), 0o644)
	_ = afero.WriteFile(dst, "/out/ch1/0002.jpg", []byte("old"), 0o644)
	_ = afero.WriteFile(dst, "/out/ch2/0001.jpg", []byte("old"), 0o644)
	_ = afero.WriteFile(dst, "/out/ch2/0002.jpg", []byte("old"), 0o644)

	src := afero.NewMemMapFs()
	_ = afero.WriteFile(src, "/d/ch1/0001.png", []byte("new"), 0o644)
	_ = afero.WriteFile(src, "/d/ch2/0001.jpg", []byte("new"), 0o644)
	replace := func(path string) bool { return path == "/out/ch1" }
	if err := mergeDirectories(0o755, 0o644, dst, "/out", src, "/d", replace); err != nil {
		t.Fatal(err)
	}
	for dir, want := range map[string][]string{
		"/out":     {"ch1", "ch2"},
		"/out/ch1": {"0001.png"},
		"/out/ch2": {"0001.jpg", "0002.jpg"},
	} {
		entries, _ := afero.ReadDir(dst, dir)
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", dir, got, want)
		}
	}
}
