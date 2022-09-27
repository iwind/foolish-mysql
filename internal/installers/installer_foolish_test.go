// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package installers_test

import (
	"foolishmysql/internal/installers"
	"testing"
)

func TestFoolishInstaller_InstallFromFile(t *testing.T) {
	var installer = installers.NewFoolishInstaller()
	err := installer.InstallFromFile("/Users/liuxiangchao/Downloads/mysql-8.0.30-linux-glibc2.17-x86_64-minimal.tar.xz", "./mysql-dir")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFoolishInstaller_Download(t *testing.T) {
	var installer = installers.NewFoolishInstaller()
	path, err := installer.Download()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("path:", path)
}
