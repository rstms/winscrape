package assets

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	//"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type CacheOption int

// UseCache - if cache exists use it and leave it in place, otherwise create and delete on close
// WriteCache - delete any existing cache and write a new one
// DeleteCache - delete any existing cache

const (
	UseCache CacheOption = iota
	WriteCache
	DeleteCache
)

func (o *CacheOption) String() string {
	return []string{"UseCache", "WriteCache", "DeleteCache"}[*o]
}

var binDirPattern = regexp.MustCompile(`^bin$`)
var dataDirPattern = regexp.MustCompile(`^tessdata$`)
var driveLetterPattern = regexp.MustCompile(`^([a-zA-Z]):(.*)$`)
var bashPattern = regexp.MustCompile(`^.*[Bb][Aa][Ss][Hh](\.[Ee][Xx][Ee]){0,1}$`)

//go:embed assets
var assets embed.FS

type Assets struct {
	Name     string
	CacheDir string
	BinDir   string
	DataDir  string
	Env      map[string]string
	Option   CacheOption
	depth    int
	dirty    bool
	debug    bool
}

func CreateCache() (string, error) {
	a, err := newAssets(WriteCache, "", []string{})
	if err != nil {
		return "", Fatal(err)
	}
	return a.Exports(), nil
}

func DestroyCache() error {
	_, err := newAssets(DeleteCache, "", []string{})
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func NewAssets() (*Assets, error) {
	a, err := newAssets(UseCache, "", []string{})
	if err != nil {
		return nil, Fatal(err)
	}
	err = a.Setenv()
	if err != nil {
		return nil, Fatal(err)
	}
	return a, nil
}

func getName() (string, error) {
	name, err := os.Executable()
	if err != nil {
		return "", Fatal(err)
	}
	name = filepath.Base(name)
	name, _, _ = strings.Cut(name, ".")
	return name, nil
}

func newAssets(option CacheOption, command string, args []string) (*Assets, error) {

	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, Fatal(err)
	}

	name, err := getName()
	if err != nil {
		return nil, Fatal(err)
	}

	a := Assets{
		Name:     name,
		CacheDir: filepath.Join(cache, name),
		Env:      map[string]string{},
		Option:   option,
		debug:    ViperGetBool("debug"),
	}

	a.Env[strings.ToUpper(a.Name)+"_CACHE"] = a.CacheDir

	err = a.generate()
	if err != nil {
		return nil, Fatal(err)
	}

	//_, configured := os.LookupEnv("TESSDATA_PREFIX")

	err = filepath.WalkDir(a.CacheDir, a.envWalker)
	if err != nil {
		return nil, Fatal(err)
	}

	if a.debug {
		log.Printf("Assets: %s\n", FormatJSON(a))
	}

	/*
		if a.Option == UseCache && !configured {
			exitCode, err := a.Exec(command, args)
			if err != nil {
				return nil, Fatal(err)
			}
			err = a.Close()
			if err != nil {
				return nil, Fatal(err)
			}
			os.Exit(exitCode)
		}
	*/

	return &a, nil
}

func (a *Assets) Exports() string {
	values := []string{}
	for key, value := range a.Env {
		if key == "PATH" {
			value = fmtPaths(value)
		} else {
			value = fmtPath(value)
		}
		export := fmt.Sprintf("export %s=%s", key, value)
		values = append(values, export)
	}
	return strings.Join(values, "\n") + "\n"
}

func (a *Assets) Setenv() error {
	for key, value := range a.Env {
		if a.debug {
			log.Printf("Setenv: %s=%s\n", key, value)
		}
		err := os.Setenv(key, value)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (a *Assets) Close() error {
	if a.dirty {
		err := a.deleteCache()
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (a *Assets) deleteCache() error {

	if a.CacheDir != "" && IsDir(a.CacheDir) {
		if a.debug {
			log.Printf("deleting cache: %s\n", a.CacheDir)
		}
		err := filepath.WalkDir(a.CacheDir, a.chmodWalker)
		if err != nil {
			return Fatal(err)
		}
		err = os.RemoveAll(a.CacheDir)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (a *Assets) generate() error {

	switch a.Option {
	case UseCache:
		if IsDir(a.CacheDir) {
			if a.debug {
				log.Printf("using cache: %s\n", a.CacheDir)
			}
			return nil
		}
		// delete cache on close
		a.dirty = true

	case DeleteCache, WriteCache:
		err := a.deleteCache()
		if err != nil {
			return Fatal(err)
		}
	default:
		return Fatalf("unexpected option: %v", a.Option)
	}

	if a.Option == DeleteCache {
		return nil
	}

	if a.debug {
		log.Printf("writing cache: %s\n", a.CacheDir)
	}

	err := fs.WalkDir(assets, "assets", a.genWalker)
	if err != nil {
		return Fatal(err)
	}

	return nil
}

func (a *Assets) genWalker(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	dest := filepath.Join(a.CacheDir, path)
	switch {
	case d.IsDir():
		err := os.MkdirAll(dest, d.Type())
		if err != nil {
			return Fatal(err)
		}
	case d.Type().IsRegular():
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
	}
	return nil
}

func (a *Assets) envWalker(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	switch {
	case d.IsDir() && a.BinDir == "" && binDirPattern.MatchString(d.Name()):
		a.BinDir = filepath.Clean(path)
		if a.debug {
			log.Printf("BinDir: %s\n", a.BinDir)
		}
		value, err := addPath(a.BinDir)
		if err != nil {
			return Fatal(err)
		}
		a.Env["PATH"] = value
	case d.IsDir() && a.DataDir == "" && dataDirPattern.MatchString(d.Name()):
		a.DataDir = filepath.Clean(path)
		if a.debug {
			log.Printf("DataDir: %s\n", a.DataDir)
		}
		a.Env["TESSDATA_PREFIX"] = a.DataDir + string(os.PathSeparator)
		//case a.BinDir != "" && a.DataDir != "":
		//	return fs.SkipAll
	}
	return nil
}

func (a *Assets) chmodWalker(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	switch {
	case d.IsDir():
		//log.Printf("chmod 0777 %s\n", path)
		err := os.Chmod(path, 0777)
		if err != nil {
			return Fatal(err)
		}
	case d.Type().IsRegular():
		//log.Printf("chmod 0666 %s\n", path)
		err := os.Chmod(path, 0666)
		if err != nil {
			return Fatal(err)
		}
	default:
		return Fatalf("unexpected type: %s %v", path, d)
	}
	return nil
}

func isBash() bool {
	return bashPattern.MatchString(os.Getenv("SHELL"))
}

func fmtPath(path string) string {
	if isBash() {
		//old := path
		groups := driveLetterPattern.FindStringSubmatch(path)
		//log.Printf("%d groups=%v path=%s re=%v\n", len(groups), groups, path, driveLetterPattern)
		if len(groups) == 3 {
			path = "/" + groups[1] + groups[2]
		}
		path = strings.ReplaceAll(path, "\\", "/")
		//log.Printf("\nold=%s\nnew=%s\n", old, path)
	}
	return path
}

func fmtPaths(paths string) string {
	if isBash() {
		dirs := []string{}
		for _, path := range strings.Split(paths, string(os.PathListSeparator)) {
			dirs = append(dirs, fmtPath(path))
		}
		paths = strings.Join(dirs, ":")
	}
	return paths
}

func addPath(dir string) (string, error) {
	paths, ok := os.LookupEnv("PATH")
	if !ok {
		return "", Fatalf("PATH lookup failed")
	}
	if !IsDir(dir) {
		return "", Fatalf("not a directory: %s", dir)
	}
	dirs := append([]string{dir}, strings.Split(paths, string(os.PathListSeparator))...)
	return strings.Join(dirs, string(os.PathListSeparator)), nil
}

/*
func (a *Assets) isSpawned() bool {
	key := strings.ToUpper(a.Name) + "_ASSET_EXEC"
	value, ok := os.LookupEnv(key)
	log.Printf("pid=%d ppid=%d %s=%s\n", os.Getpid(), os.Getppid(), key, value)
	return ok
}

func (a *Assets) Exec(command string, args []string) (int, error) {

	if a.isSpawned() {
		return 0, Fatalf("refusing recursion")
	}

	binary, err := os.Executable()
	if err != nil {
		return 0, Fatal(err)
	}
	args = append([]string{command}, args...)
	cmd := exec.Command(binary, args...)
	cmd.Dir = a.BinDir
	cmd.Env = []string{fmt.Sprintf("%s_ASSET_EXEC=%d", strings.ToUpper(a.Name), os.Getpid())}
	for _, line := range os.Environ() {
		log.Printf("envline: %s\n", line)
		key, value, _ := strings.Cut(line, "=")
		log.Printf("  key=%s value=%s\n", key, value)
		_, ok := a.Env[key]
		if !ok {
			cmd.Env = append(cmd.Env, line)
		}
	}
	for k, v := range a.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if a.debug {
		log.Printf("Exec: cmd=%v\nDir=%s\n---env_start---\n%s\n---env_end---\n", cmd, cmd.Dir, strings.Join(cmd.Env, "\n"))
	}
	err = cmd.Run()
	if err != nil {
		return 0, Fatal(err)
	}
	exitCode := cmd.ProcessState.ExitCode()
	return exitCode, nil
}
*/
