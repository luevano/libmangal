package libmangal

import (
	"errors"
	"os"
	"path/filepath"
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
	err := mergeDirectories(0o755, 0o644, dstFS, dst, failingFs{src, "ch1.cbz"}, "/d")
	if !errors.Is(err, errWrite) {
		t.Fatalf("got %v, want %v", err, errWrite)
	}
	if b, err := os.ReadFile(existing); err != nil || string(b) != "old" {
		t.Errorf("existing chapter: %q, %v, want it untouched", b, err)
	}

	// a new chapter directory (images format)
	_ = afero.WriteFile(src, "/d/manga/ch2/0001.jpg", []byte("1"), 0o644)
	_ = afero.WriteFile(src, "/d/manga/ch2/0002.jpg", []byte("2"), 0o644)
	err = mergeDirectories(0o755, 0o644, dstFS, dst, failingFs{src, "0002.jpg"}, "/d")
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
	if err := mergeDirectories(0o755, 0o644, dstFS, dst, src, "/d"); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"manga/ch1.cbz": "new", "manga/ch2/0002.jpg": "2"} {
		if b, err := os.ReadFile(filepath.Join(dst, path)); err != nil || string(b) != want {
			t.Errorf("%s: %q, %v, want %q", path, b, err, want)
		}
	}
}
