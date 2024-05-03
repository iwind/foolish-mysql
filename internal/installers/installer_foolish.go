// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package installers

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"foolishmysql/internal/utils"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
		// check target dir
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

	// check 'tar' command
	this.log("checking 'tar' command ...")
	var tarExe, _ = exec.LookPath("tar")
	if len(tarExe) == 0 {
		this.log("installing 'tar' command ...")
		err = this.installTarCommand()
		if err != nil {
			this.log("WARN: failed to install 'tar' ...")
		}
	}

	// check commands
	this.log("checking system commands ...")
	var cmdList = []string{"tar" /** again **/, "chown", "sh"}
	for _, cmd := range cmdList {
		cmdPath, err := exec.LookPath(cmd)
		if err != nil || len(cmdPath) == 0 {
			return errors.New("could not find '" + cmd + "' command in this system")
		}
	}

	groupAddExe, err := this.lookupGroupAdd()
	if err != nil {
		return errors.New("could not find 'groupadd' command in this system")
	}

	userAddExe, err := this.lookupUserAdd()
	if err != nil {
		return errors.New("could not find 'useradd' command in this system")
	}

	// ubuntu apt
	aptGetExe, err := exec.LookPath("apt-get")
	if err == nil && len(aptGetExe) > 0 {
		for _, lib := range []string{"libaio1", "libncurses5"} {
			this.log("checking " + lib + " ...")
			var cmd = utils.NewCmd(aptGetExe, "-y", "install", lib)
			cmd.WithStderr()
			err = cmd.Run()
			if err != nil {
				// try apt
				aptExe, aptErr := exec.LookPath("apt")
				if aptErr == nil && len(aptExe) > 0 {
					cmd = utils.NewCmd(aptExe, "-y", "install", lib)
					cmd.WithStderr()
					err = cmd.Run()
				}

				if err != nil {
					return errors.New("install " + lib + " failed: " + cmd.Stderr())
				}
			}
			time.Sleep(1 * time.Second)
		}
	} else { // yum
		yumExe, err := exec.LookPath("yum")
		if err == nil && len(yumExe) > 0 {
			for _, lib := range []string{"libaio", "ncurses-libs", "ncurses-compat-libs", "numactl-libs"} {
				var cmd = utils.NewCmd("yum", "-y", "install", lib)
				_ = cmd.Run()
				time.Sleep(1 * time.Second)
			}
		}

		// create symbolic links
		{
			var libFile = "/usr/lib64/libncurses.so.5"
			_, err = os.Stat(libFile)
			if err != nil && os.IsNotExist(err) {
				var latestLibFile = utils.FindLatestVersionFile("/usr/lib64", "libncurses.so.")
				if len(latestLibFile) > 0 {
					this.log("link '" + latestLibFile + "' to '" + libFile + "'")
					_ = os.Symlink(latestLibFile, libFile)
				}
			}
		}

		{
			var libFile = "/usr/lib64/libtinfo.so.5"
			_, err = os.Stat(libFile)
			if err != nil && os.IsNotExist(err) {
				var latestLibFile = utils.FindLatestVersionFile("/usr/lib64", "libtinfo.so.")
				if len(latestLibFile) > 0 {
					this.log("link '" + latestLibFile + "' to '" + libFile + "'")
					_ = os.Symlink(latestLibFile, libFile)
				}
			}
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
			var cmd = utils.NewCmd(groupAddExe, "mysql")
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
			var cmd *utils.Cmd
			if strings.HasSuffix(userAddExe, "useradd") {
				cmd = utils.NewCmd(userAddExe, "mysql", "-g", "mysql")
			} else { // adduser
				cmd = utils.NewCmd(userAddExe, "-S", "-G", "mysql", "mysql")
			}
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
		err = os.WriteFile(passwordFile, []byte(generatedPassword+"\n"), 0666)
		if err != nil {
			return errors.New("write password failed: " + err.Error())
		}
	}

	// move to right place
	this.log("moving files to target dir ...")
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

		// waiting for startup
		for i := 0; i < 30; i++ {
			var conn net.Conn
			conn, err = net.Dial("tcp", "127.0.0.1:3306")
			if err != nil {
				time.Sleep(1 * time.Second)
			} else {
				_ = conn.Close()
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	// change password
	newPassword, err := this.generatePassword()
	if err != nil {
		return errors.New("generate new password failed: " + err.Error())
	}

	this.log("changing mysql password ...")
	var passwordSQL = "ALTER USER 'root'@'localhost' IDENTIFIED BY '" + newPassword + "';"
	{
		var cmd = utils.NewCmd("sh", "-c", baseDir+"/bin/mysql --host=\"127.0.0.1\"  --user=root --password=\""+generatedPassword+"\" --execute=\""+passwordSQL+"\" --connect-expired-password")
		cmd.WithStderr()
		err = cmd.Run()
		if err != nil {
			return errors.New("change password failed: " + cmd.String() + ": " + cmd.Stderr())
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

	// create link to 'mysql' client command
	var clientExe = "/usr/local/bin/mysql"
	_, err = os.Stat(clientExe)
	if err != nil && os.IsNotExist(err) {
		err = os.Symlink(baseDir+"/bin/mysql", clientExe)
		if err == nil {
			this.log("created symbolic link '" + clientExe + "' to '" + baseDir + "/bin/mysql'")
		} else {
			this.log("WARN: failed to create symbolic link '" + clientExe + "' to '" + baseDir + "/bin/mysql': " + err.Error())
		}
	}

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
	var latestVersion = "8.2.0" // default version
	var majorVersion = "8.2"
	{
		req, reqErr := http.NewRequest(http.MethodGet, "https://dev.mysql.com/downloads/mysql/", nil)
		if reqErr != nil {
			return "", reqErr
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Ubuntu Chromium/78.0.3904.108 Chrome/78.0.3904.108 Safari/537.36")
		resp, err := client.Do(req)
		if err != nil {
			return "", errors.New("check latest version failed: " + err.Error())
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", errors.New("read latest version failed: " + err.Error())
			}

			var reg = regexp.MustCompile(`<h1>MySQL Community Server ([\d.]+) `)
			var matches = reg.FindSubmatch(data)
			if len(matches) > 0 {
				latestVersion = string(matches[1])
				var matchPieces = strings.Split(latestVersion, ".")
				if len(matchPieces) >= 2 {
					majorVersion = strings.Join(matchPieces[:2], ".")
				}
			}
		}
	}
	this.log("found version: v" + latestVersion)

	// download
	this.log("start downloading ...")

	var downloadURL = "https://cdn.mysql.com/Downloads/MySQL-" + majorVersion + "/mysql-" + latestVersion + "-linux-glibc2.17-x86_64-minimal.tar.xz"

	{
		this.log("downloading from url '" + downloadURL + "' ...")
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
	var memoryTotalG = 1

	if runtime.GOOS == "linux" {
		memoryTotalG = utils.SysMemoryGB() / 2
		if memoryTotalG <= 0 {
			memoryTotalG = 1
		}
	}

	return `
[mysqld]
port=3306
basedir="` + baseDir + `"
datadir="` + dataDir + `"

max_connections=256
innodb_flush_log_at_trx_commit=2
max_prepared_stmt_count=65535
binlog_cache_size=1M
binlog_stmt_cache_size=1M
thread_cache_size=32
binlog_expire_logs_seconds=604800
innodb_sort_buffer_size=8M
innodb_buffer_pool_size=` + strconv.Itoa(memoryTotalG) + "G"
}

// generate random password
func (this *FoolishInstaller) generatePassword() (string, error) {
	var p = make([]byte, 16)
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

	var startCmd = "${BASE_DIR}/support-files/mysql.server start"
	bashPath, _ := exec.LookPath("bash")
	if len(bashPath) > 0 {
		startCmd = bashPath + " -c \"" + startCmd + "\""
	}

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
Restart=on-failure
RestartSec=5s
RemainAfterExit=yes
ExecStart=` + startCmd + `
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

// install 'tar' command automatically
func (this *FoolishInstaller) installTarCommand() error {
	// dnf
	dnfExe, err := exec.LookPath("dnf")
	if err == nil && len(dnfExe) > 0 {
		var cmd = utils.NewTimeoutCmd(10*time.Second, dnfExe, "-y", "install", "tar")
		return cmd.Run()
	}

	// yum
	yumExe, err := exec.LookPath("yum")
	if err == nil && len(yumExe) > 0 {
		var cmd = utils.NewTimeoutCmd(10*time.Second, yumExe, "-y", "install", "tar")
		return cmd.Run()
	}

	// apt-get
	aptGetExe, err := exec.LookPath("apt-get")
	if err == nil && len(aptGetExe) > 0 {
		var cmd = utils.NewTimeoutCmd(10*time.Second, aptGetExe, "-y", "install", "tar")
		err = cmd.Run()
		if err != nil {
			aptExe, aptErr := exec.LookPath("apt")
			if aptErr == nil {
				cmd = utils.NewTimeoutCmd(10*time.Second, aptExe, "-y", "install", "tar")
				err = cmd.Run()
			}
		}
		return err
	}

	return nil
}

func (this *FoolishInstaller) lookupGroupAdd() (string, error) {
	for _, cmd := range []string{"groupadd", "addgroup"} {
		path, err := exec.LookPath(cmd)
		if err == nil && len(path) > 0 {
			return path, nil
		}
	}
	return "", errors.New("not found")
}

func (this *FoolishInstaller) lookupUserAdd() (string, error) {
	for _, cmd := range []string{"useradd", "adduser"} {
		path, err := exec.LookPath(cmd)
		if err == nil && len(path) > 0 {
			return path, nil
		}
	}
	return "", errors.New("not found")
}
