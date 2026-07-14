// Package main generates Windows resource manifests.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const resourceTemplate = `{
  "RT_GROUP_ICON": {
    "#2": {
      "0000": [
        "icon_256x256.png"
      ]
    }
  },
  "RT_MANIFEST": {
    "#1": {
      "0409": {
        "identity": {
          "name": "%s",
          "version": "%s"
        },
        "description": "Go Music DL desktop application",
        "minimum-os": "vista",
        "execution-level": "as invoker",
        "ui-access": false,
        "auto-elevate": false,
        "dpi-awareness": "system",
        "disable-theming": false,
        "disable-window-filtering": false,
        "high-resolution-scrolling-aware": false,
        "ultra-high-resolution-scrolling-aware": false,
        "long-path-aware": false,
        "printer-driver-isolation": false,
        "gdi-scaling": false,
        "segment-heap": false,
        "use-common-controls-v6": false
      }
    }
  },
  "RT_VERSION": {
    "#1": {
      "0000": {
        "fixed": {
          "file_version": "%s",
          "product_version": "%s",
          "timestamp": "%s"
        },
        "info": {
          "0409": {
            "Comments": "A complete, engineered Go music download project with CLI and Web interface",
            "CompanyName": "guohuiyuan",
            "FileDescription": "https://github.com/guohuiyuan/go-music-dl",
            "FileVersion": "%s",
            "InternalName": "%s",
            "LegalCopyright": "%s",
            "LegalTrademarks": "",
            "OriginalFilename": "%s",
            "PrivateBuild": "",
            "ProductName": "Go Music DL",
            "ProductVersion": "%s",
            "SpecialBuild": ""
          }
        }
      }
    }
  }
}`

const timeFormat = "2006-01-02T15:04:05+08:00"

type resourceSpec struct {
	outputFile       string
	identityName     string
	internalName     string
	originalFileName string
}

func main() {
	fileVersion := resolveFileVersion()
	productVersion := "v1.0.0"
	timestamp := time.Now().Format(timeFormat)
	copyright := "(c) 2026 guohuiyuan. All Rights Reserved."

	specs := []resourceSpec{
		{
			outputFile:       "winres.json",
			identityName:     "go-music-dl-desktop",
			internalName:     "go-music-dl-desktop",
			originalFileName: "go-music-dl-desktop.exe",
		},
		{
			outputFile:       "desktop_go.winres.json",
			identityName:     "music-dl-desktop-go",
			internalName:     "music-dl-desktop-go",
			originalFileName: "music-dl-desktop-go.exe",
		},
	}

	for _, spec := range specs {
		if err := writeResourceFile(spec, fileVersion, productVersion, timestamp, copyright); err != nil {
			panic(err)
		}
	}
}

func resolveFileVersion() string {
	var stdout strings.Builder
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "1.0.0.0"
	}

	commitCount := strings.TrimSpace(stdout.String())
	if commitCount == "" {
		return "1.0.0.0"
	}

	return "1.0.0." + commitCount
}

func writeResourceFile(
	spec resourceSpec,
	fileVersion string,
	productVersion string,
	timestamp string,
	copyright string,
) error {
	f, err := os.Create(spec.outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(
		f,
		resourceTemplate,
		spec.identityName,
		fileVersion,
		fileVersion,
		productVersion,
		timestamp,
		fileVersion,
		spec.internalName,
		copyright,
		spec.originalFileName,
		productVersion,
	)
	return err
}
