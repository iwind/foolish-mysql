// Copyright 2023 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package utils_test

import (
	"foolishmysql/internal/utils"
	"testing"
)

func TestSysMemoryGB(t *testing.T) {
	t.Log(utils.SysMemoryGB())
}
