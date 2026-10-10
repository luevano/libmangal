package libmangal

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// partSuffix marks files and dirs still being written.
const partSuffix = ".part"

// mergeDirectories merges two directories recursively from different filesystems.
// If a file exists in both directories it will be overwritten.
//
// Files are written next to their destination and renamed into place, and new
// directories are filled before being renamed, so an interrupted merge never leaves
// a partial file (or chapter directory) under its final name.
func mergeDirectories(
	modeDir, modeFile fs.FileMode,
	dstFS afero.Fs, dstDir string,
	srcFS afero.Fs, srcDir string,
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
		if exists {
			if err := mergeDirectories(modeDir, modeFile, dstFS, dstPath, srcFS, srcPath); err != nil {
				return err
			}
			continue
		}

		tmpPath := dstPath + partSuffix
		if err := dstFS.RemoveAll(tmpPath); err != nil {
			return err
		}
		if err := mergeDirectories(modeDir, modeFile, dstFS, tmpPath, srcFS, srcPath); err != nil {
			return errors.Join(err, dstFS.RemoveAll(tmpPath))
		}
		if err := dstFS.Rename(tmpPath, dstPath); err != nil {
			return errors.Join(err, dstFS.RemoveAll(tmpPath))
		}
	}

	return nil
}

// copyFile copies src to a temporary file next to dst, then renames it to dst.
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

	tmpPath := dstPath + partSuffix
	tmp, err := dstFS.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, modeFile)
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
	if err := tmp.Close(); err != nil {
		return err
	}
	return dstFS.Rename(tmpPath, dstPath)
}
