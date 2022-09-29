// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package main

import (
	"fmt"
	"foolishmysql/internal/installers"
	"github.com/fatih/color"
	"os"
	"path/filepath"
)

func main() {
	var args = os.Args
	if len(args) > 1 {
		var cmd = args[1]
		if cmd == "-v" || cmd == "--version" || cmd == "version" {
			fmt.Println(installers.Version)
			return
		}
	}

	var installer = installers.NewFoolishInstaller()
	var targetDir = "/usr/local/mysql"

	// check target dir
	_, err := os.Stat(targetDir)
	if err == nil {
		// 检查是否为空
		matches, _ := filepath.Glob(targetDir + "/*")
		if len(matches) > 0 {
			_, _ = color.New(color.FgRed).Println("target dir '" + targetDir + "' already exists and not empty, please check if you are using the directory")
			return
		}
	}

	var xzFile string
	if len(args) == 1 {
		xzFile, err = installer.Download()
		if err != nil {
			_, _ = color.New(color.FgRed).Println("download failed: " + err.Error())
			return
		}
	} else if len(args) == 2 {
		xzFile = args[1]
	}

	if len(xzFile) == 0 {
		fmt.Println("usage: ./foolish-mysql or ./foolish-mysql XZ_FILE")
		return
	}

	err = installer.InstallFromFile(xzFile, targetDir)
	if err != nil {
		_, _ = color.New(color.FgRed).Println("install from file '" + xzFile + "' failed: " + err.Error())
	} else {
		_, _ = color.New(color.FgGreen).Println("installed successfully\n=======\nuser: root\npassword: " + installer.Password() + "\ndir: " + targetDir)
	}
}
