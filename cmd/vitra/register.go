package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.klarlabs.de/vitra/platform"
)

func registerScheme(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vitra register-scheme <scheme> [app-id] [exec]")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return fmt.Errorf("register-scheme is only implemented on Linux (xdg), Darwin (helper .app), and Windows (.reg)")
	}
	scheme := args[0]
	appID := "com.vitra.app"
	if len(args) > 1 {
		appID = args[1]
	}
	execPath := ""
	if len(args) > 2 {
		execPath = args[2]
	} else {
		var err error
		execPath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	host := currentHost()
	reg, ok := host.(platform.URLSchemeRegistrar)
	if !ok {
		return &platform.ErrUnsupported{Feature: platform.FeatureDeepLink, OS: host.OS(), Detail: "host does not implement platform.URLSchemeRegistrar"}
	}
	if err := reg.RegisterURLScheme(scheme, appID, execPath); err != nil {
		return err
	}
	fmt.Printf("registered URL handler for %s:// → %s (%s)\n", scheme, execPath, appID)
	return nil
}

func registerFiles(args []string) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return fmt.Errorf("register-files is only implemented on Linux (xdg), Darwin (helper .app), and Windows (.reg)")
	}
	var mimes []string
	appID := "com.vitra.app"
	name := ""
	execPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mime":
			i++
			if i >= len(args) {
				return fmt.Errorf("--mime requires a type")
			}
			mimes = append(mimes, args[i])
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires a value")
			}
			appID = args[i]
		case "--exec":
			i++
			if i >= len(args) {
				return fmt.Errorf("--exec requires a path")
			}
			execPath = args[i]
		case "--name":
			i++
			if i >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			name = args[i]
		default:
			return fmt.Errorf("unknown register-files flag %q", args[i])
		}
	}
	if len(mimes) == 0 {
		return fmt.Errorf("usage: vitra register-files --mime <type> [--mime <type>] [--app-id id] [--exec path] [--name name]")
	}
	if execPath == "" {
		var err error
		execPath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	if name == "" {
		name = appID
	}
	host := currentHost()
	reg, ok := host.(platform.FileAssociationRegistrar)
	if !ok {
		return &platform.ErrUnsupported{Feature: platform.FeatureFileAssociation, OS: host.OS(), Detail: "host does not implement platform.FileAssociationRegistrar"}
	}
	if err := reg.RegisterFileAssociations(appID, execPath, name, mimes); err != nil {
		return err
	}
	fmt.Printf("registered file associations %s → %s (%s)\n", strings.Join(mimes, ","), execPath, appID)
	return nil
}
