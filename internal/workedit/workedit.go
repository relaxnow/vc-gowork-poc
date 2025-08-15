package workedit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vc-gowork-poc/internal/copytree"
	"vc-gowork-poc/internal/util"

	"golang.org/x/mod/modfile"
)

// RewriteGoWorkFiles updates use and path-based replace entries.
// External paths are copied under externalBase. Returns a set of directories
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

		workFile, err := modfile.ParseWork("go.work", data, nil)
		if err != nil {
			return nil, err
		}

		var updatedUsePaths []string

		for _, use := range workFile.Use {
			origUseAbs := use.Path
			if !filepath.IsAbs(origUseAbs) {
				origUseAbs = filepath.Clean(filepath.Join(workDirOriginal, use.Path))
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
				newRel := filepath.ToSlash(relFromWorkToCopied)
				fmt.Printf("[work] %s: use %q -> %q (inside source tree)\n", workPathCopied, use.Path, newRel)
				use.Path = newRel
				updatedUsePaths = append(updatedUsePaths, newRel)
				usedModuleDirs[filepath.Clean(copiedAbs)] = struct{}{}
			} else {
				base := filepath.Base(origUseAbs)
				destDir := util.UniqueDir(filepath.Join(externalBase, base))
				fmt.Printf("[work] %s: use %q external -> copying to %s\n", workPathCopied, use.Path, destDir)
				if err := copytree.CopyTreeNormalized(origUseAbs, destDir); err != nil {
					return nil, err
				}
				relFromWorkToDest, err := filepath.Rel(workDirCopied, destDir)
				if err != nil {
					return nil, err
				}
				newRel := filepath.ToSlash(relFromWorkToDest)
				fmt.Printf("[work] %s: use %q -> %q (copied external)\n", workPathCopied, use.Path, newRel)
				use.Path = newRel
				updatedUsePaths = append(updatedUsePaths, newRel)
				usedModuleDirs[filepath.Clean(destDir)] = struct{}{}
			}
		}

		for _, rep := range workFile.Replace {
			if rep.New.Version != "" || rep.New.Path == "" {
				continue
			}
			origNewAbs := rep.New.Path
			if !filepath.IsAbs(origNewAbs) {
				origNewAbs = filepath.Clean(filepath.Join(workDirOriginal, rep.New.Path))
			}

			var targetAbs string
			if util.IsWithin(origNewAbs, originalRoot) {
				relFromOriginalRoot, err := filepath.Rel(originalRoot, origNewAbs)
				if err != nil {
					return nil, err
				}
				targetAbs = filepath.Join(copiedRoot, relFromOriginalRoot)
				fmt.Printf("[work] %s: replace %q => %q (inside source tree)\n", workPathCopied, rep.New.Path, targetAbs)
			} else {
				base := filepath.Base(origNewAbs)
				destDir := util.UniqueDir(filepath.Join(externalBase, base))
				fmt.Printf("[work] %s: replace %q external -> copying to %s\n", workPathCopied, rep.New.Path, destDir)
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
			fmt.Printf("[work] %s: replace %q => %q (final path in go.work)\n", workPathCopied, rep.New.Path, finalRel)
			rep.New.Path = finalRel
			rep.New.Version = ""
		}

		outBytes := renderGoWork(workFile, updatedUsePaths)
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

// renderGoWork creates a minimal, correct go.work with updated use and replace entries.
func renderGoWork(wf *modfile.WorkFile, updatedUsePaths []string) []byte {
	var b strings.Builder

	if wf.Go != nil && strings.TrimSpace(wf.Go.Version) != "" {
		fmt.Fprintf(&b, "go %s\n\n", strings.TrimSpace(wf.Go.Version))
	}

	b.WriteString("use (\n")
	for _, up := range updatedUsePaths {
		up = strings.TrimSpace(up)
		if up == "" {
			continue
		}
		fmt.Fprintf(&b, "\t%s\n", filepath.ToSlash(up))
	}
	b.WriteString(")\n")

	hadReplace := false
	for _, r := range wf.Replace {
		if r.New.Path == "" || r.New.Version != "" {
			continue
		}
		if !hadReplace {
			b.WriteString("\nreplace (\n")
			hadReplace = true
		}
		if strings.TrimSpace(r.Old.Version) != "" {
			fmt.Fprintf(&b, "\t%s %s => %s\n", r.Old.Path, r.Old.Version, filepath.ToSlash(r.New.Path))
		} else {
			fmt.Fprintf(&b, "\t%s => %s\n", r.Old.Path, filepath.ToSlash(r.New.Path))
		}
	}
	if hadReplace {
		b.WriteString(")\n")
	}

	b.WriteString("\n")
	return []byte(b.String())
}
