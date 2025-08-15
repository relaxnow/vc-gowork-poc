package vendorstep

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"vc-gowork-poc/internal/util"
)

// RunVendorSteps runs vendoring with tidying first.
// - For each go.work file directory:
//   - Run "go mod tidy" in every module dir that appears in usedModuleDirs (and has a go.mod)
//   - Run "go work vendor" in the go.work directory
//
// - For each go.mod not covered by any go.work use:
//   - Run "go mod tidy" then "go mod vendor"
func RunVendorSteps(workFiles []string, modFiles []string, usedModuleDirs map[string]struct{}) error {
	// 1) For each workspace, tidy all used modules first, then vendor at the workspace root.
	for _, workPath := range workFiles {
		workDir := filepath.Dir(workPath)

		// Tidy all modules referenced by any go.work use entry
		for modDir := range usedModuleDirs {
			// Only tidy those that actually exist and contain a go.mod file
			if fileExists(filepath.Join(modDir, "go.mod")) {
				fmt.Printf("[work] tidy in %s (referenced by go.work)\n", modDir)
				if err := runCmd(modDir, "go", "mod", "tidy"); err != nil {
					fmt.Fprintf(os.Stderr, "warning: go mod tidy failed in %s: %v\n", modDir, err)
				}
			}
		}

		// Now vendor at the workspace root
		fmt.Printf("[work] vendor in %s\n", workDir)
		if err := runCmd(workDir, "go", "work", "vendor"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: go work vendor failed in %s: %v\n", workDir, err)
		}
	}

	// 2) For standalone modules not covered by any go.work use, tidy then vendor.
	skipModDirs := make(map[string]struct{}, len(usedModuleDirs))
	for d := range usedModuleDirs {
		skipModDirs[filepath.Clean(d)] = struct{}{}
	}
	for _, modPath := range modFiles {
		modDir := filepath.Dir(modPath)
		if util.IsUnderAny(modDir, skipModDirs) {
			fmt.Printf("[mod ] vendor skipped for %s (covered by go.work use)\n", modDir)
			continue
		}

		// Tidy then vendor
		fmt.Printf("[mod ] tidy in %s\n", modDir)
		if err := runCmd(modDir, "go", "mod", "tidy"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: go mod tidy failed in %s: %v\n", modDir, err)
		}

		fmt.Printf("[mod ] vendor in %s\n", modDir)
		if err := runCmd(modDir, "go", "mod", "vendor"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: go mod vendor failed in %s: %v\n", modDir, err)
		}
	}

	return nil
}

/* ---------- helpers ---------- */

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = util.Stdout()
	cmd.Stderr = util.Stderr()
	return cmd.Run()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
