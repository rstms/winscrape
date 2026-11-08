package tesseract

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// subdirectory containing tesseract dll and data directory

//go:embed assets
var assets embed.FS

var BINARY_PATTERN = regexp.MustCompile(`^assets[\\/]bin[\\/]tesseract.exe$`)
var DATA_PATTERN = regexp.MustCompile(`^assets[\\/]tessdata$`)

type Tesseract struct {
	TempDir string
	BinDir  string
	DataDir string
	debug   bool
}

func NewTesseract() (*Tesseract, error) {
	t := Tesseract{
		debug: ViperGetBool("debug"),
	}
	err := t.generateAssets()
	if err != nil {
		return nil, Fatal(err)
	}
	log.Printf("Tesseract: %s\n", FormatJSON(t))
	os.Setenv("TESSDATA_PREFIX", t.DataDir)
	path, err := addPath(t.BinDir)
	if err != nil {
		return nil, Fatal(err)
	}
	os.Setenv("PATH", path)
	return &t, nil
}

func (t *Tesseract) Close() error {
	if t.TempDir != "" {
		err := os.RemoveAll(t.TempDir)
		if err != nil {
			return Fatal(err)
		}
		t.TempDir = ""
	}
	return nil
}

// copy embedded assets to temp dir
func (t *Tesseract) generateAssets() error {
	var err error
	t.TempDir, err = os.MkdirTemp("", "tmp.*")
	if err != nil {
		return Fatal(err)
	}
	err = fs.WalkDir(assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(t.TempDir, path)
		if d.IsDir() {
			err := os.Mkdir(dest, d.Type())
			if err != nil {
				return Fatal(err)
			}
			if DATA_PATTERN.MatchString(path) && t.DataDir == "" {
				t.DataDir = filepath.Clean(dest)
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
			if BINARY_PATTERN.MatchString(path) && t.BinDir == "" {
				t.BinDir, _ = filepath.Split(filepath.Clean(dest))
				t.BinDir = strings.TrimRight(t.BinDir, string(os.PathSeparator))
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func addPath(dir string) (string, error) {
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
