package zipper

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ZipDirFilteredIncludeRoot zips srcDir as a top-level folder into destZip.
// Only includes *.go, *.gotmpl, go.mod, go.sum, modules.txt, go.work.
// Symlinks are stored with their target as file content.
func ZipDirFilteredIncludeRoot(srcDir string, destZip string) error {
	allow := func(relName string) bool {
		base := filepath.Base(relName)
		switch base {
		case "go.mod", "go.sum", "modules.txt", "go.work":
			return true
		}
		ext := strings.ToLower(filepath.Ext(base))
		return ext == ".go" || ext == ".gotmpl"
	}

	if err := os.MkdirAll(filepath.Dir(destZip), 0o755); err != nil {
		return err
	}
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	parent := filepath.Dir(srcDir)
	return filepath.WalkDir(parent, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relFromParent, err := filepath.Rel(parent, currentPath)
		if err != nil {
			return err
		}
		if relFromParent == "." {
			return nil
		}
		name := filepath.ToSlash(relFromParent)

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if entry.IsDir() {
			h, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			h.Name = name + "/"
			_, err = zw.CreateHeader(h)
			return err
		}

		if !allow(name) {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(currentPath)
			if err != nil {
				return err
			}
			h, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			h.Name = name
			h.SetMode(os.ModeSymlink | 0o777)
			w, err := zw.CreateHeader(h)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, target)
			return err
		}

		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = name
		h.Method = zip.Deflate

		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		f, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}
