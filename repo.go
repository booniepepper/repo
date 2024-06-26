package main

import (
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"lesiw.io/flag"
)

var repodir = "."
var errParse = errors.New("parse error")

//go:embed version.txt
var versionfile string

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, errParse) {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		os.Exit(1)
	}
}

func run() error {
	defer func() { fmt.Println(repodir) }()

	flags := flag.NewFlagSet(os.Stderr, "repo")
	flags.Usage = "Usage: repo URL"
	version := flags.Bool("V,version", "print version and exit")
	force := flags.Bool("f,force", "delete and re-clone repository")
	if err := flags.Parse(os.Args[1:]...); err != nil {
		return errParse
	}
	if len(flags.Args) == 0 {
		flags.PrintError("no URL given")
		return errParse
	}
	if *version {
		return fmt.Errorf(strings.TrimSpace(versionfile))
	}

	rawurl := flags.Args[0]
	parts, err := urlToPath(rawurl)
	if err != nil {
		return err
	}
	prefix := os.Getenv("REPOPREFIX")
	if prefix == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not get home directory: %s", err)
		}
		prefix = filepath.Join(home, ".local", "src")
	}

	dirpath := filepath.Join(parts[:len(parts)-1]...)
	dirpath = filepath.Join(prefix, dirpath)
	fullpath := filepath.Join(parts...)
	fullpath = filepath.Join(prefix, fullpath)
	if info, err := os.Stat(fullpath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("'%s' exists and is not a directory", err)
		}
		if *force {
			if err := os.RemoveAll(fullpath); err != nil {
				return fmt.Errorf("could not remove directory '%s': %s",
					fullpath, err)
			}
		} else {
			repodir = fullpath
			return nil
		}
	}

	tmpdir, err := os.MkdirTemp("", "repo.*")
	if err != nil {
		return fmt.Errorf("could not make temp directory: %s", err)
	}
	defer os.RemoveAll(tmpdir)
	if err := cloneRepo(rawurl, tmpdir); err != nil {
		return fmt.Errorf("could not clone repository: %s", err)
	}

	if err := os.MkdirAll(dirpath, 0755); err != nil {
		return fmt.Errorf("could not create repo directory: %s", err)
	}
	if err := os.Rename(tmpdir, fullpath); err != nil {
		return fmt.Errorf("could not move cloned repository to '%s': %s",
			fullpath, err)
	}

	repodir = fullpath
	return nil
}

func urlToPath(rawurl string) (path []string, err error) {
	var repo *url.URL
	if _, rest, ok := strings.Cut(rawurl, "@"); ok {
		rawurl = rest
		rawurl = strings.Replace(rawurl, ":", "/", 1)
	}

	if repo, err = url.Parse(rawurl); err != nil {
		return
	}
	if repo.Hostname() != "" {
		path = append(path, repo.Hostname())
	}

	rawpath := repo.Path
	if rest, ok := strings.CutSuffix(rawpath, ".git"); ok {
		rawpath = rest
	}

	for _, p := range strings.Split(rawpath, "/") {
		if p != "" {
			path = append(path, p)
		}
	}
	if len(path) < 1 {
		err = fmt.Errorf("failed to derive path from url: %s", rawurl)
	}
	return
}

func cloneRepo(loc string, path string) error {
	cmd := exec.Command("git", "clone", loc, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr // Stdout should only ever contain repodir.
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
