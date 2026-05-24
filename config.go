package context

import (
	"fmt"
	"path/filepath"

	"github.com/sds-framework/os-lib/path"
)

// Specifically for Dev Context
const (
	// SrcKey is the path of source directory from the configuration
	SrcKey = "SERVICE_DEPS_SRC"
	// BinKey is the path of bin directory from the configuration
	BinKey = "SERVICE_DEPS_BIN"
)

// DevDefaultPaths returns the required developer context's local paths.
//
// It sets the source manager's bin path and source path in (dot is current dir by executable):
//
//		/bin.exe
//		/_sds/source/
//		/_sds/bin/
//	/_sds/source/github.com.ahmetson.proxy-lib/main.go
//	/_sds/bin/github.com.ahmetson.proxy-lib.exe
func DevDefaultPaths() (string, string, error) {
	currentDir, err := path.CurrentDir()
	if err != nil {
		return "", "", fmt.Errorf("path.CurrentDir: %w", err)
	}

	srcPath := filepath.Join(currentDir, "_sds", "src")
	binPath := filepath.Join(currentDir, "_sds", "bin")

	return srcPath, binPath, nil
}
