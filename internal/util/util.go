package util

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

func PanicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func Stdout() *os.File { return os.Stdout }
func Stderr() *os.File { return os.Stderr }

func FindWorkAndModFiles(root string) (workFiles []string, modFiles []string, err error) {
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Base(p) {
		case "go.work":
			workFiles = append(workFiles, p)
		case "go.mod":
			modFiles = append(modFiles, p)
		}
		return nil
	})
	return
}

func IsWithin(path string, root string) bool {
	absPath, err1 := filepath.Abs(path)
	absRoot, err2 := filepath.Abs(root)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || !hasDotDotPrefix(rel)
}

func IsUnderAny(dir string, set map[string]struct{}) bool {
	dir = filepath.Clean(dir)
	for used := range set {
		used = filepath.Clean(used)
		if dir == used {
			return true
		}
		rel, err := filepath.Rel(used, dir)
		if err == nil && rel != "." && !hasDotDotPrefix(rel) {
			return true
		}
	}
	return false
}

func UniqueDir(base string) string {
	tryPath := base
	index := 1
	for {
		_, err := os.Stat(tryPath)
		if errors.Is(err, os.ErrNotExist) {
			return tryPath
		}
		tryPath = base + "-" + strconv.Itoa(index)
		index++
	}
}

func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 2 && rel[:2] == ".."
}
