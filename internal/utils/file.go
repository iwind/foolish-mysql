// Copyright 2023 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package utils

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func FindLatestVersionFile(dir string, prefix string) string {
	files, err := filepath.Glob(filepath.Clean(dir + "/" + prefix + "*"))
	if err != nil {
		return ""
	}
	var resultFile = ""
	var lastVersion = ""
	var reg = regexp.MustCompile(`\.([\d.]+)`)
	for _, file := range files {
		var filename = filepath.Base(file)
		var matches = reg.FindStringSubmatch(filename)
		if len(matches) > 1 {
			var version = matches[1]
			if len(lastVersion) == 0 || VersionCompare(lastVersion, version) < 0 {
				lastVersion = version
				resultFile = file
			}
		}
	}

	return resultFile
}

// VersionCompare compare two versions
func VersionCompare(version1 string, version2 string) int8 {
	if len(version1) == 0 {
		if len(version2) == 0 {
			return 0
		}

		return -1
	}

	if len(version2) == 0 {
		return 1
	}

	pieces1 := strings.Split(version1, ".")
	pieces2 := strings.Split(version2, ".")
	count1 := len(pieces1)
	count2 := len(pieces2)

	for i := 0; i < count1; i++ {
		if i > count2-1 {
			return 1
		}

		piece1 := pieces1[i]
		piece2 := pieces2[i]
		len1 := len(piece1)
		len2 := len(piece2)

		if len1 == 0 {
			if len2 == 0 {
				continue
			}
		}

		maxLength := 0
		if len1 > len2 {
			maxLength = len1
		} else {
			maxLength = len2
		}

		piece1 = fmt.Sprintf("%0"+strconv.Itoa(maxLength)+"s", piece1)
		piece2 = fmt.Sprintf("%0"+strconv.Itoa(maxLength)+"s", piece2)

		if piece1 > piece2 {
			return 1
		}

		if piece1 < piece2 {
			return -1
		}
	}

	if count1 > count2 {
		return 1
	}

	if count1 == count2 {
		return 0
	}

	return -1
}
