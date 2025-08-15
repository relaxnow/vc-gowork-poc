package vendorstep

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"vc-gowork-poc/internal/util"
)

func RunVendorSteps(workFiles []string, modFiles []string, usedModuleDirs map[string]struct{}) error {
	for _, workPath := range workFiles {
		dir := filepath.Dir(workPath)
		fmt.Printf("[work] vendor in %s\n", dir)
		cmd := exec.Command("go", "work", "vendor")
		cmd.Dir = dir
		cmd.Stdout = util.Stdout()
		cmd.Stderr = util.Stderr()
		if err := cmd.Run(); err != nil {
			fmt.Printf("warning: go work vendor failed in %s: %v\n", dir, err)
		}
	}

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
		fmt.Printf("[mod ] vendor in %s\n", modDir)
		cmd := exec.Command("go", "mod", "vendor")
		cmd.Dir = modDir
		cmd.Stdout = util.Stdout()
		cmd.Stderr = util.Stderr()
		if err := cmd.Run(); err != nil {
			fmt.Printf("warning: go mod vendor failed in %s: %v\n", modDir, err)
		}
	}
	return nil
}
