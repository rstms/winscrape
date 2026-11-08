package cmd

import (
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// subdirectory containing tesseract dll and data directory

//go:embed assets
var assets embed.FS

var BINARY_PATTERN = regexp.MustCompile(`^assets[\\/]bin[\\/]winocr.exe$`)
var DATA_PATTERN = regexp.MustCompile(`^assets[\\/]tessdata$`)

// copy assets to temp dir, return binFile, dataDir, tempDir
func GenerateAssets() (string, string, string, error) {
	var binFile string
	var dataDir string
	tmpDir, err := os.MkdirTemp("", "tmp.*")
	if err != nil {
		return "", "", "", Fatal(err)
	}
	err = fs.WalkDir(assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(tmpDir, path)
		if d.IsDir() {
			err := os.Mkdir(dest, d.Type())
			if err != nil {
				return Fatal(err)
			}
			if DATA_PATTERN.MatchString(path) && dataDir == "" {
				dataDir = filepath.Clean(dest)
			}
			return nil
		}
		if d.Type().IsRegular() {
			ifp, err := assets.Open(path)
			if err != nil {
				return Fatal(err)
			}
			defer ifp.Close()
			ofp, err := os.Create(dest)
			if err != nil {
				return Fatal(err)
			}
			defer ofp.Close()
			_, err = io.Copy(ofp, ifp)
			if err != nil {
				return Fatal(err)
			}
			err = os.Chmod(dest, d.Type())
			if err != nil {
				return Fatal(err)
			}
			if BINARY_PATTERN.MatchString(path) && binFile == "" {
				binFile = filepath.Clean(dest)
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return "", "", "", Fatal(err)
	}
	return binFile, dataDir, tmpDir, nil
}

func AddPath(dir string) (string, error) {
	path, ok := os.LookupEnv("PATH")
	if !ok {
		return "", Fatalf("PATH lookup failed")
	}
	if !IsDir(dir) {
		return "", Fatalf("not a directory: %s", dir)
	}
	dirs := strings.Split(path, string(os.PathListSeparator))
	dirs = append([]string{filepath.Clean(dir)}, dirs...)
	path = strings.Join(dirs, string(os.PathListSeparator))
	return path, nil
}
