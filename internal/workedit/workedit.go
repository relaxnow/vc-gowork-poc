package workedit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/relaxnow/vc-gowork-poc/internal/copytree"
	"github.com/relaxnow/vc-gowork-poc/internal/util"

	"golang.org/x/mod/modfile"
)

// ReplaceEdit describes one replace directive to add.
type ReplaceEdit struct {
	oldPath, oldVersion string
	newPath, newVersion string
}

// RewriteGoWorkFiles updates use and path-based replace entries IN PLACE.
// External paths are copied under externalBase. It returns a set of directories
// referenced by use directives after rewrite.
func RewriteGoWorkFiles(originalRoot, copiedRoot string, workFiles []string, externalBase string) (map[string]struct{}, error) {
	usedModuleDirs := make(map[string]struct{})

	for _, workPathCopied := range workFiles {
		workDirCopied := filepath.Dir(workPathCopied)
		relFromCopiedRoot, err := filepath.Rel(copiedRoot, workDirCopied)
		if err != nil {
			return nil, err
		}
		workDirOriginal := filepath.Join(originalRoot, relFromCopiedRoot)

		data, err := os.ReadFile(workPathCopied)
		if err != nil {
			return nil, err
		}

		wf, err := modfile.ParseWork("go.work", data, nil)
		if err != nil {
			return nil, err
		}

		// Build desired new state without mutating wf.Use or wf.Replace yet.
		var desiredUsePaths []string
		var desiredReplaces []ReplaceEdit

		// Compute desired USE entries
		for _, u := range wf.Use {
			origUseAbs := u.Path
			if !filepath.IsAbs(origUseAbs) {
				origUseAbs = filepath.Clean(filepath.Join(workDirOriginal, u.Path))
			}
			if util.IsWithin(origUseAbs, originalRoot) {
				relFromOriginalRoot, err := filepath.Rel(originalRoot, origUseAbs)
				if err != nil {
					return nil, err
				}
				copiedAbs := filepath.Join(copiedRoot, relFromOriginalRoot)
				relFromWorkToCopied, err := filepath.Rel(workDirCopied, copiedAbs)
				if err != nil {
					return nil, err
				}
				final := filepath.ToSlash(relFromWorkToCopied)

				origToken := filepath.ToSlash(u.Path)
				if !filepath.IsAbs(u.Path) && (origToken == final || origToken == ".") {
					desiredUsePaths = append(desiredUsePaths, origToken)
				} else {
					fmt.Printf("[work] %s: use %q -> %q (normalize inside tree)\n",
						workPathCopied, u.Path, final)
					desiredUsePaths = append(desiredUsePaths, final)
				}
				usedModuleDirs[filepath.Clean(copiedAbs)] = struct{}{}
			} else {
				base := filepath.Base(origUseAbs)
				destDir := util.UniqueDir(filepath.Join(externalBase, base))
				fmt.Printf("[work] %s: use %q external -> copying to %s\n",
					workPathCopied, u.Path, destDir)
				if err := copytree.CopyTreeNormalized(origUseAbs, destDir); err != nil {
					return nil, err
				}
				relFromWorkToDest, err := filepath.Rel(workDirCopied, destDir)
				if err != nil {
					return nil, err
				}
				final := filepath.ToSlash(relFromWorkToDest)
				fmt.Printf("[work] %s: use %q -> %q (copied external)\n",
					workPathCopied, u.Path, final)
				desiredUsePaths = append(desiredUsePaths, final)
				usedModuleDirs[filepath.Clean(destDir)] = struct{}{}
			}
		}

		// Compute desired REPLACE entries (versionless, path-based)
		for _, r := range wf.Replace {
			if r.New.Version != "" || r.New.Path == "" {
				continue
			}
			origNewAbs := r.New.Path
			if !filepath.IsAbs(origNewAbs) {
				origNewAbs = filepath.Clean(filepath.Join(workDirOriginal, r.New.Path))
			}

			var targetAbs string
			if util.IsWithin(origNewAbs, originalRoot) {
				relFromOriginalRoot, err := filepath.Rel(originalRoot, origNewAbs)
				if err != nil {
					return nil, err
				}
				targetAbs = filepath.Join(copiedRoot, relFromOriginalRoot)
				fmt.Printf("[work] %s: replace %q => %q (inside source tree)\n",
					workPathCopied, r.New.Path, targetAbs)
			} else {
				base := filepath.Base(origNewAbs)
				destDir := util.UniqueDir(filepath.Join(externalBase, base))
				fmt.Printf("[work] %s: replace %q external -> copying to %s\n",
					workPathCopied, r.New.Path, destDir)
				if err := copytree.CopyTreeNormalized(origNewAbs, destDir); err != nil {
					return nil, err
				}
				targetAbs = destDir
			}

			relFromWorkToTarget, err := filepath.Rel(workDirCopied, targetAbs)
			if err != nil {
				return nil, err
			}
			finalRel := filepath.ToSlash(relFromWorkToTarget)
			fmt.Printf("[work] %s: replace %q => %q (final path in go.work)\n",
				workPathCopied, r.New.Path, finalRel)

			desiredReplaces = append(desiredReplaces, ReplaceEdit{
				oldPath: r.Old.Path, oldVersion: r.Old.Version,
				newPath: finalRel, newVersion: "",
			})
			// Do not mutate r.New.Path here.
		}

		// Apply edits in place and write back
		outBytes := renderGoWorkInPlace(wf, desiredUsePaths, desiredReplaces)
		if err := os.WriteFile(workPathCopied, outBytes, 0o644); err != nil {
			return nil, err
		}
	}

	return usedModuleDirs, nil
}

// RewriteGoModFiles updates path-based replaces in go.mod files.
func RewriteGoModFiles(originalRoot, copiedRoot string, modFiles []string, externalBase string) error {
	for _, modPathCopied := range modFiles {
		modDirCopied := filepath.Dir(modPathCopied)
		relFromCopiedRoot, err := filepath.Rel(copiedRoot, modDirCopied)
		if err != nil {
			return err
		}
		modDirOriginal := filepath.Join(originalRoot, relFromCopiedRoot)

		data, err := os.ReadFile(modPathCopied)
		if err != nil {
			return err
		}

		modFile, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			return err
		}

		changed := false
		for _, rep := range modFile.Replace {
			if rep.New.Version != "" || rep.New.Path == "" {
				continue
			}
			origNewAbs := rep.New.Path
			if !filepath.IsAbs(origNewAbs) {
				origNewAbs = filepath.Clean(filepath.Join(modDirOriginal, rep.New.Path))
			}

			var targetAbs string
			if util.IsWithin(origNewAbs, originalRoot) {
				relFromOriginalRoot, err := filepath.Rel(originalRoot, origNewAbs)
				if err != nil {
					return err
				}
				targetAbs = filepath.Join(copiedRoot, relFromOriginalRoot)
				fmt.Printf("[mod ] %s: replace %q => %q (inside source tree)\n", modPathCopied, rep.New.Path, targetAbs)
			} else {
				base := filepath.Base(origNewAbs)
				destDir := util.UniqueDir(filepath.Join(externalBase, base))
				fmt.Printf("[mod ] %s: replace %q external -> copying to %s\n", modPathCopied, rep.New.Path, destDir)
				if err := copytree.CopyTreeNormalized(origNewAbs, destDir); err != nil {
					return err
				}
				targetAbs = destDir
			}

			relFromModToTarget, err := filepath.Rel(modDirCopied, targetAbs)
			if err != nil {
				return err
			}
			finalRel := filepath.ToSlash(relFromModToTarget)
			fmt.Printf("[mod ] %s: replace %q => %q (final path in go.mod)\n", modPathCopied, rep.New.Path, finalRel)

			rep.New.Path = finalRel
			rep.New.Version = ""
			changed = true
		}

		if changed {
			formatted, err := modFile.Format()
			if err != nil {
				return err
			}
			if err := os.WriteFile(modPathCopied, formatted, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// renderGoWorkInPlace replaces all existing USE and path-based REPLACE entries
// on the provided WorkFile using public helpers, then returns modfile.Format.
// It panics on any helper error.
func renderGoWorkInPlace(wf *modfile.WorkFile, desiredUsePaths []string, desiredReplaces []ReplaceEdit) []byte {
	// Normalize, dedupe, sort uses deterministically
	seen := make(map[string]struct{}, len(desiredUsePaths))
	cleanUses := make([]string, 0, len(desiredUsePaths))
	for _, p := range desiredUsePaths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			cleanUses = append(cleanUses, p)
		}
	}
	sort.Strings(cleanUses)

	// Collect original tokens BEFORE any edits
	var origUses []string
	for _, u := range wf.Use {
		origUses = append(origUses, filepath.ToSlash(strings.TrimSpace(u.Path)))
	}
	type orep struct{ oldPath, oldVersion string }
	var origRepls []orep
	for _, r := range wf.Replace {
		origRepls = append(origRepls, orep{oldPath: r.Old.Path, oldVersion: r.Old.Version})
	}

	// Drop all original uses
	for _, up := range origUses {
		if err := wf.DropUse(up); err != nil {
			panic(err)
		}
	}
	// Drop all original replaces
	for _, or := range origRepls {
		if err := wf.DropReplace(or.oldPath, or.oldVersion); err != nil {
			panic(err)
		}
	}

	// Add desired uses
	for _, p := range cleanUses {
		if err := wf.AddUse(p, p); err != nil {
			panic(err)
		}
	}

	// Add desired replaces
	for _, nr := range desiredReplaces {
		if err := wf.AddReplace(nr.oldPath, nr.oldVersion, nr.newPath, nr.newVersion); err != nil {
			panic(err)
		}
	}

	// Serialize updated syntax tree
	return modfile.Format(wf.Syntax)
}
