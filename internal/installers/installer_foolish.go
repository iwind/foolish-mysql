// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package installers

import (
	"bytes"
	"errors"
	"fmt"
	"foolishmysql/internal/utils"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FoolishInstaller struct {
	password string
}

func NewFoolishInstaller() *FoolishInstaller {
	return &FoolishInstaller{}
}

func (this *FoolishInstaller) InstallFromFile(xzFilePath string, targetDir string) error {
	// check whether mysql already running
	this.log("checking mysqld ...")
	var oldPid = utils.FindPidWithName("mysqld")
	if oldPid > 0 {
		return errors.New("there is already a running mysql server process, pid: '" + strconv.Itoa(oldPid) + "'")
	}

	// check target dir
	this.log("checking target dir '" + targetDir + "' ...")
	_, err := os.Stat(targetDir)
	if err == nil {
		// 检查是否为空
		matches, _ := filepath.Glob(targetDir + "/*")
		if len(matches) > 0 {
			return errors.New("target dir '" + targetDir + "' already exists and not empty")
		} else {
			err = os.Remove(targetDir)
			if err != nil {
				return errors.New("clean target dir '" + targetDir + "' failed: " + err.Error())
			}
		}
	}

	// check commands
	this.log("checking system commands ...")
	var cmdList = []string{"tar", "groupadd", "useradd", "chown", "sh"}
	for _, cmd := range cmdList {
		cmdPath, err := exec.LookPath(cmd)
		if err != nil || len(cmdPath) == 0 {
			return errors.New("could not find '" + cmd + "' command in this system")
		}
	}

	// create 'mysql' user group
	this.log("checking 'mysql' user group ...")
	{
		data, err := os.ReadFile("/etc/group")
		if err != nil {
			return errors.New("check user group failed: " + err.Error())
		}
		if !bytes.Contains(data, []byte("\nmysql:")) {
			var cmd = utils.NewCmd("groupadd", "mysql")
			cmd.WithStderr()
			err = cmd.Run()
			if err != nil {
				return errors.New("add 'mysql' user group failed: " + cmd.Stderr())
			}
		}
	}

	// create 'mysql' user
	this.log("checking 'mysql' user ...")
	{
		data, err := os.ReadFile("/etc/passwd")
		if err != nil {
			return errors.New("check user failed: " + err.Error())
		}
		if !bytes.Contains(data, []byte("\nmysql:")) {
			var cmd = utils.NewCmd("useradd", "mysql", "-g", "mysql")
			cmd.WithStderr()
			err = cmd.Run()
			if err != nil {
				return errors.New("add 'mysql' user failed: " + cmd.Stderr())
			}
		}
	}

	// mkdir
	{
		var parentDir = filepath.Dir(targetDir)
		stat, err := os.Stat(parentDir)
		if err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(parentDir, 0777)
				if err != nil {
					return errors.New("try to create dir '" + parentDir + "' failed: " + err.Error())
				}
			} else {
				return errors.New("check dir '" + parentDir + "' failed: " + err.Error())
			}
		} else {
			if !stat.IsDir() {
				return errors.New("'" + parentDir + "' should be a directory")
			}
		}
	}

	// check installer file .xz
	this.log("checking installer file ...")
	{
		stat, err := os.Stat(xzFilePath)
		if err != nil {
			return errors.New("could not open the installer file: " + err.Error())
		}
		if stat.IsDir() {
			return errors.New("'" + xzFilePath + "' not a valid file")
		}

		var basename = filepath.Base(xzFilePath)
		if !strings.HasSuffix(basename, ".xz") {
			return errors.New("installer file should has '.xz' extension")
		}
	}

	// extract
	this.log("extracting installer file ...")
	var tmpDir = os.TempDir() + "/foolish-mysql-tmp"
	{
		_, err := os.Stat(tmpDir)
		if err == nil {
			err = os.RemoveAll(tmpDir)
			if err != nil {
				return errors.New("clean temporary directory '" + tmpDir + "' failed: " + err.Error())
			}
		}
		err = os.Mkdir(tmpDir, 0777)
		if err != nil {
			return errors.New("create temporary directory '" + tmpDir + "' failed: " + err.Error())
		}
	}

	{
		var cmd = utils.NewCmd("tar", "-xJvf", xzFilePath, "-C", tmpDir)
		cmd.WithStderr()
		err = cmd.Run()
		if err != nil {
			return errors.New("extract installer file '" + xzFilePath + "' failed: " + cmd.Stderr())
		}
	}

	// create datadir
	matches, err := filepath.Glob(tmpDir + "/mysql-*")
	if err != nil || len(matches) == 0 {
		return errors.New("could not find mysql installer directory from '" + tmpDir + "'")
	}
	var baseDir = matches[0]
	var dataDir = baseDir + "/data"
	_, err = os.Stat(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(dataDir, 0777)
			if err != nil {
				return errors.New("create data dir '" + dataDir + "' failed: " + err.Error())
			}
		} else {
			return errors.New("check data dir '" + dataDir + "' failed: " + err.Error())
		}
	}

	// chown datadir
	{
		var cmd = utils.NewCmd("chown", "mysql:mysql", dataDir)
		cmd.WithStderr()
		err = cmd.Run()
		if err != nil {
			return errors.New("chown data dir '" + dataDir + "' failed: " + err.Error())
		}
	}

	// create my.cnf
	var myCnfFile = "/etc/my.cnf"
	_, err = os.Stat(myCnfFile)
	if err == nil {
		// backup it
		err = os.Rename(myCnfFile, "/etc/my.cnf."+utils.Format("YmdHis"))
		if err != nil {
			return errors.New("backup '/etc/my.cnf' failed: " + err.Error())
		}
	}

	// mysql server options https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html
	var myCnfTemplate = this.createMyCnf(baseDir, dataDir)
	err = os.WriteFile(myCnfFile, []byte(myCnfTemplate), 0666)
	if err != nil {
		return errors.New("write '" + myCnfFile + "' failed: " + err.Error())
	}

	// initialize
	this.log("initializing mysql ...")
	var generatedPassword = ""
	{
		var cmd = utils.NewCmd(baseDir+"/bin/mysqld", "--initialize", "--user=mysql")
		cmd.WithStderr()
		cmd.WithStdout()
		err = cmd.Run()
		if err != nil {
			return errors.New("initialize failed: " + cmd.Stderr())
		}

		// read from stdout
		var match = regexp.MustCompile(`temporary password is.+:\s*(.+)`).FindStringSubmatch(cmd.Stdout())
		if len(match) == 0 {
			// read from stderr
			match = regexp.MustCompile(`temporary password is.+:\s*(.+)`).FindStringSubmatch(cmd.Stderr())

			if len(match) == 0 {
				return errors.New("initialize successfully, but could not find generated password, please report to developer")
			}
		}
		generatedPassword = strings.TrimSpace(match[1])

		// write password to file
		var passwordFile = baseDir + "/generated-password.txt"
		err = os.WriteFile(passwordFile, []byte(generatedPassword), 0666)
		if err != nil {
			return errors.New("write password failed: " + err.Error())
		}
	}

	// move to right place
	this.log("move files to target dir ...")
	err = os.Rename(baseDir, targetDir)
	if err != nil {
		return errors.New("move '" + baseDir + "' to '" + targetDir + "' failed: " + err.Error())
	}
	baseDir = targetDir

	// change my.cnf
	myCnfTemplate = this.createMyCnf(baseDir, baseDir+"/data")
	err = os.WriteFile(myCnfFile, []byte(myCnfTemplate), 0666)
	if err != nil {
		return errors.New("create new '" + myCnfFile + "' failed: " + err.Error())
	}

	// start mysql
	this.log("starting mysql ...")
	{
		var cmd = utils.NewCmd(baseDir+"/bin/mysqld_safe", "--user=mysql")
		cmd.WithStderr()
		err = cmd.Start()
		if err != nil {
			return errors.New("start failed '" + cmd.String() + "': " + cmd.Stderr())
		}
	}

	// change password
	newPassword, err := this.generatePassword()
	if err != nil {
		return errors.New("generate new password failed: " + err.Error())
	}

	this.log("changing mysql password ...")
	var passwordSQL = "ALTER USER 'root'@'localhost' IDENTIFIED BY '" + newPassword + "';"
	var changePasswordTries = 3
	for i := 0; i < changePasswordTries; i++ {
		var cmd = utils.NewCmd("sh", "-c", baseDir+"/bin/mysql --user=root --password=\""+generatedPassword+"\" --execute=\""+passwordSQL+"\" --connect-expired-password")
		cmd.WithStderr()
		err = cmd.Run()
		if err != nil {
			if i == changePasswordTries-1 {

				return errors.New("change password failed: " + cmd.String() + ": " + cmd.Stderr())
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}
	this.password = newPassword
	var passwordFile = baseDir + "/generated-password.txt"
	err = os.WriteFile(passwordFile, []byte(this.password), 0666)
	if err != nil {
		return errors.New("write generated file failed: " + err.Error())
	}

	// remove temporary directory
	_ = os.Remove(tmpDir)

	// install service
	// this is not required, so we ignore all errors
	err = this.installService(baseDir)
	if err != nil {
		this.log("WARN: install service failed: " + err.Error())
	}

	this.log("finished")

	return nil
}

func (this *FoolishInstaller) Download() (path string, err error) {
	var client = &http.Client{}

	// check latest version
	this.log("checking mysql latest version ...")
	var latestVersion = ""
	{
		req, err := http.NewRequest(http.MethodGet, "https://dev.mysql.com/downloads/mysql/", nil)
		if err != nil {
			return "", err
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36")
		resp, err := client.Do(req)
		if err != nil {
			return "", errors.New("check latest version failed: " + err.Error())
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return "", errors.New("check latest version failed: invalid response code: " + strconv.Itoa(resp.StatusCode))
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", errors.New("read latest version failed: " + err.Error())
		}

		var reg = regexp.MustCompile(`<h1>MySQL Community Server ([\d.]+) </h1>`)
		var matches = reg.FindSubmatch(data)
		if len(matches) == 0 {
			return "", errors.New("parse latest version failed, please report to developer")
		}
		latestVersion = string(matches[1])
	}
	this.log("found version: v" + latestVersion)

	// download
	this.log("start downloading ...")
	var downloadURL = "https://cdn.mysql.com//Downloads/MySQL-8.0/mysql-" + latestVersion + "-linux-glibc2.17-x86_64-minimal.tar.xz"

	{
		req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
		if err != nil {
			return "", err
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36")
		resp, err := client.Do(req)
		if err != nil {
			return "", errors.New("check latest version failed: " + err.Error())
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return "", errors.New("check latest version failed: invalid response code: " + strconv.Itoa(resp.StatusCode))
		}

		path = filepath.Base(downloadURL)
		fp, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if err != nil {
			return "", errors.New("create download file '" + path + "' failed: " + err.Error())
		}
		var writer = utils.NewProgressWriter(fp, resp.ContentLength)
		var ticker = time.NewTicker(1 * time.Second)
		var done = make(chan bool, 1)
		go func() {
			var lastProgress float32 = -1

			for {
				select {
				case <-ticker.C:
					var progress = writer.Progress()
					if lastProgress < 0 || progress-lastProgress > 0.1 || progress == 1 {
						lastProgress = progress
						this.log(fmt.Sprintf("%.2f%%", progress*100))
					}
				case <-done:
					return
				}
			}
		}()
		_, err = io.Copy(writer, resp.Body)
		if err != nil {
			_ = fp.Close()
			done <- true
			return "", errors.New("download failed: " + err.Error())
		}

		err = fp.Close()
		if err != nil {
			done <- true
			return "", errors.New("download failed: " + err.Error())
		}

		time.Sleep(1 * time.Second) // waiting for progress printing
		done <- true
	}

	return path, nil
}

// Password get generated password
func (this *FoolishInstaller) Password() string {
	return this.password
}

// create my.cnf content
func (this *FoolishInstaller) createMyCnf(baseDir string, dataDir string) string {
	return `
[mysqld]
port=3306
basedir="` + baseDir + `"
datadir="` + dataDir + `"

`
}

// generate random password
func (this *FoolishInstaller) generatePassword() (string, error) {
	var p = make([]byte, 16)
	rand.Seed(time.Now().UnixNano())
	n, err := rand.Read(p)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", p[:n]), nil
}

// print log
func (this *FoolishInstaller) log(message string) {
	_, b := os.LookupEnv("QUIET")
	if b {
		return
	}
	log.Println(message)
}

// copy file
func (this *FoolishInstaller) installService(baseDir string) error {
	_, err := exec.LookPath("systemctl")
	if err != nil {
		return err
	}

	this.log("registering systemd service ...")

	var desc = `### BEGIN INIT INFO
# Provides: mysql
# Required-Start: $local_fs $network $remote_fs
# Should-Start: ypbind nscd ldap ntpd xntpd
# Required-Stop: $local_fs $network $remote_fs
# Default-Start:  2 3 4 5
# Default-Stop: 0 1 6
# Short-Description: start and stop MySQL
# Description: MySQL is a very fast and reliable SQL database engine.
### END INIT INFO

[Unit]
Description=MySQL Service
Before=shutdown.target
After=network-online.target

[Service]
Type=simple
Restart=always
RestartSec=1s
ExecStart=${BASE_DIR}/support-files/mysql.server start
ExecStop=${BASE_DIR}/support-files/mysql.server stop
ExecRestart=${BASE_DIR}/support-files/mysql.server restart
ExecStatus=${BASE_DIR}/support-files/mysql.server status
ExecReload=${BASE_DIR}/support-files/mysql.server reload

[Install]
WantedBy=multi-user.target`

	desc = strings.ReplaceAll(desc, "${BASE_DIR}", baseDir)

	err = os.WriteFile("/etc/systemd/system/mysqld.service", []byte(desc), 0666)
	if err != nil {
		return err
	}

	var cmd = utils.NewTimeoutCmd(5*time.Second, "systemctl", "enable", "mysqld.service")
	cmd.WithStderr()
	err = cmd.Run()
	if err != nil {
		return errors.New("enable mysqld.service failed: " + cmd.Stderr())
	}

	return nil
}