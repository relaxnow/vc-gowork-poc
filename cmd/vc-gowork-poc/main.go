package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/relaxnow/vc-gowork-poc/internal/copytree"
	"github.com/relaxnow/vc-gowork-poc/internal/util"
	"github.com/relaxnow/vc-gowork-poc/internal/vendorstep"
	"github.com/relaxnow/vc-gowork-poc/internal/workedit"
	"github.com/relaxnow/vc-gowork-poc/internal/zipper"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <directory>\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	originalRoot, err := filepath.Abs(os.Args[1])
	util.PanicOnErr(err)

	tempRoot, err := os.MkdirTemp("", "vc-gowork-poc-")
	util.PanicOnErr(err)
	// Comment the following to keep files on disk for debugging:
	defer func() { _ = os.RemoveAll(tempRoot) }()

	// Copy source into temp workspace
	copiedRoot := filepath.Join(tempRoot, filepath.Base(originalRoot))
	util.PanicOnErr(copytree.CopyTreeNormalized(originalRoot, copiedRoot))
	fmt.Printf("[copy] %s -> %s\n", originalRoot, copiedRoot)

	// Discover go.work and go.mod
	workFiles, modFiles, err := util.FindWorkAndModFiles(copiedRoot)
	util.PanicOnErr(err)
	fmt.Printf("[scan] found %d go.work, %d go.mod\n", len(workFiles), len(modFiles))

	// Rewrite go.work and go.mod
	externalBase := filepath.Join(copiedRoot, "_external")
	util.PanicOnErr(os.MkdirAll(externalBase, 0o755))

	usedModuleDirs, err := workedit.RewriteGoWorkFiles(originalRoot, copiedRoot, workFiles, externalBase)
	util.PanicOnErr(err)

	util.PanicOnErr(workedit.RewriteGoModFiles(originalRoot, copiedRoot, modFiles, externalBase))

	// Vendor
	util.PanicOnErr(vendorstep.RunVendorSteps(workFiles, modFiles, usedModuleDirs))

	// Zip with filter, include root folder
	cwd, err := os.Getwd()
	util.PanicOnErr(err)
	outZip := filepath.Join(cwd, filepath.Base(copiedRoot)+".zip")
	fmt.Printf("[zip ] creating %s\n", outZip)
	util.PanicOnErr(zipper.ZipDirFilteredIncludeRoot(copiedRoot, outZip))

	fmt.Println("Packaging completed")
}
