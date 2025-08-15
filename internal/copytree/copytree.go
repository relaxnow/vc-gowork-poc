package copytree

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"vc-gowork-poc/internal/util"
)

// CopyTreeNormalized copies srcRoot into dstRoot.
// Directories are created with 0755. Files are created with 0644.
// Symlinks are recreated only if their targets resolve inside srcRoot.
// Otherwise it copies the dereferenced target (file or directory).
// Never modifies original files. Panics if a source file cannot be read.
func CopyTreeNormalized(srcRoot string, dstRoot string) error {
	srcInfo, err := os.Lstat(srcRoot)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcRoot)
	}
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return err
	}

	return filepath.WalkDir(srcRoot, func(currentSrcPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relFromSrcRoot, err := filepath.Rel(srcRoot, currentSrcPath)
		if err != nil {
			return err
		}
		if relFromSrcRoot == "." {
			return nil
		}
		currentDstPath := filepath.Join(dstRoot, relFromSrcRoot)

		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			linkTarget, err := os.Readlink(currentSrcPath)
			if err != nil {
				return err
			}
			srcSymlinkDir := filepath.Dir(currentSrcPath)
			resolvedTarget := linkTarget
			if !filepath.IsAbs(resolvedTarget) {
				resolvedTarget = filepath.Clean(filepath.Join(srcSymlinkDir, linkTarget))
			}

			if util.IsWithin(resolvedTarget, srcRoot) {
				relFromSrcRootToTarget, err := filepath.Rel(srcRoot, resolvedTarget)
				if err != nil {
					return err
				}
				copiedTargetAbs := filepath.Join(dstRoot, relFromSrcRootToTarget)

				dstSymlinkDir := filepath.Dir(currentDstPath)
				relFromDstToTarget, err := filepath.Rel(dstSymlinkDir, copiedTargetAbs)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(dstSymlinkDir, 0o755); err != nil {
					return err
				}
				fmt.Printf("[link] create symlink %s -> %s (target inside tree)\n",
					currentDstPath, relFromDstToTarget)
				return os.Symlink(relFromDstToTarget, currentDstPath)
			}

			info, statErr := os.Stat(resolvedTarget)
			if statErr != nil {
				return statErr
			}
			if info.IsDir() {
				fmt.Printf("[link] copy dir target of symlink %s -> %s (outside tree)\n",
					currentSrcPath, resolvedTarget)
				return CopyTreeNormalized(resolvedTarget, currentDstPath)
			}
			fmt.Printf("[link] copy file target of symlink %s -> %s (outside tree)\n",
				currentSrcPath, resolvedTarget)
			return copyFile0644(resolvedTarget, currentDstPath)

		case entry.IsDir():
			return os.MkdirAll(currentDstPath, 0o755)

		default:
			fmt.Printf("[file] copy %s -> %s\n", currentSrcPath, currentDstPath)
			return copyFile0644(currentSrcPath, currentDstPath)
		}
	})
}

func copyFile0644(srcPath string, dstPath string) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
