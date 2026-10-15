//go:build !windows && !darwin

package libmangal

// InvalidPathCharsOS are the chars a file name can't have on this OS.
const InvalidPathCharsOS = InvalidPathCharsUNIX
