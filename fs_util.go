package libmangal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// partSuffix marks files and dirs still being written.
const partSuffix = ".part"

// partPath is a unique temporary path in path's directory, ending in partSuffix, so two
// downloads into the same directory don't write to the same temporary file. It's short
// and hidden so it fits the name length limit whatever path's name is.
func partPath(path string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return filepath.Join(filepath.Dir(path), "."+hex.EncodeToString(b)+partSuffix)
}

// mergeDirectories merges two directories recursively from different filesystems.
// If a file exists in both directories it will be overwritten, as are the existing
// directories for which replace (may be nil) is true: those aren't merged.
//
// Files are written next to their destination and renamed into place, and new (or
// replaced) directories are filled before being renamed, so an interrupted merge never
// leaves a partial file (or chapter directory) under its final name.
func mergeDirectories(
	modeDir, modeFile fs.FileMode,
	dstFS afero.Fs, dstDir string,
	srcFS afero.Fs, srcDir string,
	replace func(dstPath string) bool,
) error {
	if err := dstFS.MkdirAll(dstDir, modeDir); err != nil {
		return err
	}

	srcFiles, err := afero.ReadDir(srcFS, srcDir)
	if err != nil {
		return err
	}

	for _, srcFile := range srcFiles {
		srcPath := filepath.Join(srcDir, srcFile.Name())
		dstPath := filepath.Join(dstDir, srcFile.Name())

		if !srcFile.IsDir() {
			if err := copyFile(modeFile, dstFS, dstPath, srcFS, srcPath); err != nil {
				return err
			}
			continue
		}

		exists, err := afero.DirExists(dstFS, dstPath)
		if err != nil {
			return err
		}
		replaced := exists && replace != nil && replace(dstPath)
		if exists && !replaced {
			if err := mergeDirectories(modeDir, modeFile, dstFS, dstPath, srcFS, srcPath, replace); err != nil {
				return err
			}
			continue
		}

		tmpPath := partPath(dstPath)
		if err := dstFS.Mkdir(tmpPath, modeDir); err != nil {
			return err
		}
		if err := mergeDirectories(modeDir, modeFile, dstFS, tmpPath, srcFS, srcPath, replace); err != nil {
			return errors.Join(err, dstFS.RemoveAll(tmpPath))
		}
		if replaced {
			// the old directory goes only once the new one is complete
			oldPath := partPath(dstPath)
			if err := dstFS.Rename(dstPath, oldPath); err != nil {
				return errors.Join(err, dstFS.RemoveAll(tmpPath))
			}
			if err := dstFS.Rename(tmpPath, dstPath); err != nil {
				return errors.Join(err, dstFS.Rename(oldPath, dstPath), dstFS.RemoveAll(tmpPath))
			}
			if err := dstFS.RemoveAll(oldPath); err != nil {
				return err
			}
			continue
		}
		if err := dstFS.Rename(tmpPath, dstPath); err != nil {
			// another download may have created it meanwhile
			if exists, _ := afero.DirExists(dstFS, dstPath); !exists {
				return errors.Join(err, dstFS.RemoveAll(tmpPath))
			}
			if err := mergeDirectories(modeDir, modeFile, dstFS, dstPath, dstFS, tmpPath, replace); err != nil {
				return errors.Join(err, dstFS.RemoveAll(tmpPath))
			}
			if err := dstFS.RemoveAll(tmpPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies src to a temporary file next to dst, syncs it, then renames it to dst.
func copyFile(
	modeFile fs.FileMode,
	dstFS afero.Fs, dstPath string,
	srcFS afero.Fs, srcPath string,
) (err error) {
	src, err := srcFS.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	tmpPath := partPath(dstPath)
	tmp, err := dstFS.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, modeFile)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, dstFS.Remove(tmpPath))
		}
	}()

	if _, err := io.Copy(tmp, src); err != nil {
		return errors.Join(err, tmp.Close())
	}
	// without it a crash right after the rename can leave an empty file under the final name
	if err := tmp.Sync(); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return dstFS.Rename(tmpPath, dstPath)
}
