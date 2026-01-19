// Package main generates winres.json for go-music-dl
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const js = `{
  "RT_GROUP_ICON": {
    "APP": {
      "0000": [
        "icon_256x256.png"
      ]
    }
  },
  "RT_MANIFEST": {
    "#1": {
      "0409": {
        "identity": {
          "name": "go-music-dl",
          "version": "%s"
        },
        "description": "Go Music DL - 一个完整的、工程化的 Go 音乐下载项目",
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
            "InternalName": "music-dl",
            "LegalCopyright": "%s",
            "LegalTrademarks": "",
            "OriginalFilename": "MUSIC-DL.EXE",
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

const timeformat = `2006-01-02T15:04:05+08:00`

func main() {
	f, err := os.Create("winres.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	
	// 获取版本信息
	version := "v1.0.0"
	
	// 获取 git 提交计数
	commitcnt := strings.Builder{}
	commitcntcmd := exec.Command("git", "rev-list", "--count", "HEAD")
	commitcntcmd.Stdout = &commitcnt
	err = commitcntcmd.Run()
	
	var fv string
	if err != nil {
		// 如果 git 命令失败，使用默认版本
		fv = "1.0.0.0"
	} else {
		commitCount := strings.TrimSpace(commitcnt.String())
		fv = "1.0.0." + commitCount
	}
	
	copyright := "© 2026 guohuiyuan. All Rights Reserved."
	
	_, err = fmt.Fprintf(f, js, fv, fv, version, time.Now().Format(timeformat), fv, copyright, version)
	if err != nil {
		panic(err)
	}
}
