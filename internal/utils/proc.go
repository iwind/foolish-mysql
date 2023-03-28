// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	ProcDir = "/proc"
)

func FindPidWithName(name string) int {
	// process name
	commFiles, err := filepath.Glob(ProcDir + "/*/comm")
	if err != nil {
		return 0
	}

	for _, commFile := range commFiles {
		data, err := os.ReadFile(commFile)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == name {
			var pieces = strings.Split(commFile, "/")
			var pid = pieces[len(pieces)-2]
			pidInt, _ := strconv.Atoi(pid)
			return pidInt
		}
	}

	return 0
}

func SysMemoryGB() int {
	if runtime.GOOS != "linux" {
		return 0
	}
	meminfoData, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range bytes.Split(meminfoData, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if bytes.Contains(line, []byte{':'}) {
			name, value, found := bytes.Cut(line, []byte{':'})
			if found {
				name = bytes.TrimSpace(name)
				if bytes.Equal(name, []byte("MemTotal")) {
					for _, unit := range []string{"gB", "mB", "kB"} {
						if bytes.Contains(value, []byte(unit)) {
							value = bytes.TrimSpace(bytes.ReplaceAll(value, []byte(unit), nil))
							valueInt, err := strconv.Atoi(string(value))
							if err != nil {
								return 0
							}
							switch unit {
							case "gB":
								return valueInt
							case "mB":
								return valueInt / 1024
							case "kB":
								return valueInt / 1024 / 1024
							}
							return 0
						}
					}
				}
			}
		}
	}

	return 0
}
