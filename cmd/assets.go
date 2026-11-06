package cmd

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

var BINARY_PATTERN = regexp.MustCompile(`^assets[\\/]bin[\\/]winocr.exe$`)
var DLL_PATTERN = regexp.MustCompile(`^assets[\\/]bin[\\/]libtesseract-.*\.dll$`)
var DATA_PATTERN = regexp.MustCompile(`^assets[\\/]tessdata$`)

// copy assets to temp dir
func GenerateAssets() (string, string, error) {
	tmpDir, err := os.MkdirTemp("", "runtime*")
	if err != nil {
		return "", "", Fatal(err)
	}
	log.Printf("assets_dir: %s\n", tmpDir)
	var dataDir string
	var binFile string
	var dllFile string
	var dataValue string
	var pathValue string
	err = fs.WalkDir(assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(tmpDir, path)
		if d.IsDir() {
			log.Printf("mkdir: src=%s dst=%s\n", path, dest)
			err := os.Mkdir(dest, d.Type())
			if err != nil {
				return Fatal(err)
			}
			if DATA_PATTERN.MatchString(path) && dataDir == "" {
				dir, _ := filepath.Split(dest)
				dataValue = filepath.Clean(dir)
				err := os.Setenv("TESSDATA_PREFIX", dataValue)
				if err != nil {
					return Fatal(err)
				}
				dataDir = dest
			}
			return nil
		}
		if d.Type().IsRegular() {
			log.Printf("copyfile: src=%s -> dst=%s\n", path, dest)
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
				binFile = dest
			}
			if DLL_PATTERN.MatchString(path) && dllFile == "" {
				dir, _ := filepath.Split(dest)
				var ok bool
				pathValue, ok = os.LookupEnv("PATH")
				if !ok {
					return Fatalf("PATH lookup failed")
				}
				paths := strings.Split(pathValue, string(os.PathListSeparator))
				paths = append([]string{filepath.Clean(dir)}, paths...)
				pathValue = strings.Join(paths, string(os.PathListSeparator))
				err = os.Setenv("PATH", pathValue)
				if err != nil {
					return Fatal(err)
				}
				dllFile = dest
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return "", "", Fatal(err)
	}
	log.Printf("dataDir: %s\n", dataDir)
	log.Printf("binFile: %s\n", binFile)
	log.Printf("dllFile: %s\n", dllFile)
	log.Printf("TESSDATA_PREFIX=%s\n", dataValue)
	log.Printf("PATH=%s\n", pathValue)

	return dataDir, binFile, nil
}
