// Copyright 2013 wetalk authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package utils implemented some useful functions.
package utils

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Unknwon/goconfig"
	"github.com/howeyc/fsnotify"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/cache"
	"github.com/astaxie/beego/orm"
	"github.com/beego/compress"
	"github.com/beego/i18n"

	"github.com/beego/wetalk/mailer"
)

const (
	APP_VER = "0.1.0.1114"
)

var (
	AppName             string
	AppVer              string
	AppHost             string
	AppUrl              string
	AppLogo             string
	EnforceRedirect     bool
	AvatarURL           string
	SecretKey           string
	IsProMode           bool
	MailUser            string
	MailFrom            string
	ActiveCodeLives     int
	ResetPwdCodeLives   int
	LoginRememberDays   int
	DateFormat          string
	DateTimeFormat      string
	DateTimeShortFormat string
	TimeZone            string
	RealtimeRenderMD    bool
	ImageSizeSmall      int
	ImageSizeMiddle     int
	ImageLinkAlphabets  []byte
	ImageXSend          bool
	ImageXSendHeader    string
	Langs               []string
	LoginMaxRetries     int
	LoginFailedBlocks   int
)

const (
	LangEnUS = iota
	LangZhCN
)

var (
	Cfg   *goconfig.ConfigFile
	Cache cache.Cache
)

var (
	GlobalConfPath   = "conf/global/app.ini"
	AppConfPath      = "conf/app.ini"
	CompressConfPath = "conf/compress.json"
)

// LoadConfig loads configuration file.
func LoadConfig() *goconfig.ConfigFile {
	var err error

	if fh, _ := os.OpenFile(AppConfPath, os.O_RDONLY|os.O_CREATE, 0600); fh != nil {
		fh.Close()
	}

	// Load configuration, set app version and log level.
	Cfg, err = goconfig.LoadConfigFile(GlobalConfPath)

	if Cfg == nil {
		Cfg, err = goconfig.LoadConfigFile(AppConfPath)
		if err != nil {
			fmt.Println("Fail to load configuration file: " + err.Error())
			os.Exit(2)
		}

	} else {
		Cfg.AppendFiles(AppConfPath)
	}

	Cfg.BlockMode = false

	// set time zone of wetalk system
	TimeZone = Cfg.MustValue("app", "time_zone", "UTC")
	if _, err := time.LoadLocation(TimeZone); err == nil {
		os.Setenv("TZ", TimeZone)
	} else {
		fmt.Println("Wrong time_zone: " + TimeZone + " " + err.Error())
		os.Exit(2)
	}

	// Trim 4th part.
	AppVer = strings.Join(strings.Split(APP_VER, ".")[:3], ".")

	beego.RunMode = Cfg.MustValue("app", "run_mode")
	beego.HttpPort = Cfg.MustInt("app", "http_port")

	IsProMode = beego.RunMode == "pro"
	if IsProMode {
		beego.SetLevel(beego.LevelInfo)
	}

	// cache system
	Cache, err = cache.NewCache("memory", `{"interval":360}`)

	// session settings
	beego.SessionOn = true
	beego.SessionProvider = Cfg.MustValue("session", "session_provider", "file")
	beego.SessionSavePath = Cfg.MustValue("session", "session_path", "sessions")
	beego.SessionName = Cfg.MustValue("session", "session_name", "wetalk_sess")
	beego.SessionCookieLifeTime = Cfg.MustInt("session", "session_life_time", 86400*30)
	beego.SessionGCMaxLifetime = 86400 * 365

	beego.EnableXSRF = true
	// xsrf token expire time
	beego.XSRFExpire = 86400 * 365

	driverName := Cfg.MustValue("orm", "driver_name", "mysql")
	dataSource := Cfg.MustValue("orm", "data_source", "root:root@/wetalk?charset=utf8&loc=UTC")
	maxIdle := Cfg.MustInt("orm", "max_idle_conn", 30)
	maxOpen := Cfg.MustInt("orm", "max_open_conn", 50)

	// set default database
	orm.RegisterDataBase("default", driverName, dataSource, maxIdle, maxOpen)
	orm.RunCommand()

	err = orm.RunSyncdb("default", false, false)
	if err != nil {
		beego.Error(err)
	}

	configWatcher()
	reloadConfig()

	settingLocales()
	settingCompress()

	return Cfg
}

func reloadConfig() {
	AppName = Cfg.MustValue("app", "app_name", "WeTalk Community")
	beego.AppName = AppName

	AppHost = Cfg.MustValue("app", "app_host", "127.0.0.1:8092")
	AppUrl = Cfg.MustValue("app", "app_url", "http://127.0.0.1:8092/")
	AppLogo = Cfg.MustValue("app", "app_logo", "/static/img/logo.gif")
	AvatarURL = Cfg.MustValue("app", "avatar_url")

	EnforceRedirect = Cfg.MustBool("app", "enforce_redirect")

	DateFormat = Cfg.MustValue("app", "date_format")
	DateTimeFormat = Cfg.MustValue("app", "datetime_format")
	DateTimeShortFormat = Cfg.MustValue("app", "datetime_short_format")

	SecretKey = Cfg.MustValue("app", "secret_key")
	if len(SecretKey) == 0 {
		fmt.Println("Please set your secret_key in app.ini file, I created one for you:\n" + GetRandomString(70))
	}

	ActiveCodeLives = Cfg.MustInt("app", "acitve_code_live_minutes", 180)
	ResetPwdCodeLives = Cfg.MustInt("app", "resetpwd_code_live_minutes", 180)

	LoginRememberDays = Cfg.MustInt("app", "login_remember_days", 7)
	LoginMaxRetries = Cfg.MustInt("app", "login_max_retries", 5)
	LoginFailedBlocks = Cfg.MustInt("app", "login_failed_blocks", 10)

	RealtimeRenderMD = Cfg.MustBool("app", "realtime_render_markdown")

	ImageSizeSmall = Cfg.MustInt("image", "image_size_small")
	ImageSizeMiddle = Cfg.MustInt("image", "image_size_middle")

	if ImageSizeSmall <= 0 {
		ImageSizeSmall = 300
	}

	if ImageSizeMiddle <= ImageSizeSmall {
		ImageSizeMiddle = ImageSizeSmall + 400
	}

	str := Cfg.MustValue("image", "image_link_alphabets")
	if len(str) == 0 {
		str = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}
	ImageLinkAlphabets = []byte(str)

	ImageXSend = Cfg.MustBool("image", "image_xsend", false)
	ImageXSendHeader = Cfg.MustValue("image", "image_xsend_header", "X-Accel-Redirect")

	MailUser = Cfg.MustValue("mailer", "mail_name", "WeTalk Community")
	MailFrom = Cfg.MustValue("mailer", "mail_from", "example@example.com")

	// set mailer connect args
	mailer.MailHost = Cfg.MustValue("mailer", "mail_host", "127.0.0.1:25")
	mailer.AuthUser = Cfg.MustValue("mailer", "mail_user", "example@example.com")
	mailer.AuthPass = Cfg.MustValue("mailer", "mail_pass", "******")

	orm.Debug = Cfg.MustBool("orm", "debug_log")
}

func settingLocales() {
	// load locales with locale_LANG.ini files
	langs := "en-US|zh-CN"
	for _, lang := range strings.Split(langs, "|") {
		lang = strings.TrimSpace(lang)
		files := []string{"conf/" + "locale_" + lang + ".ini"}
		if fh, err := os.Open(files[0]); err == nil {
			fh.Close()
		} else {
			files = nil
		}
		if err := i18n.SetMessage(lang, "conf/global/"+"locale_"+lang+".ini", files...); err != nil {
			beego.Error("Fail to set message file: " + err.Error())
			os.Exit(2)
		}
	}
	Langs = i18n.ListLangs()
}

func settingCompress() {
	setting, err := compress.LoadJsonConf(CompressConfPath, IsProMode, AppUrl)
	if err != nil {
		beego.Error(err)
		return
	}

	setting.RunCommand()

	if IsProMode {
		setting.RunCompress(true, false, true)
	}

	beego.AddFuncMap("compress_js", setting.Js.CompressJs)
	beego.AddFuncMap("compress_css", setting.Css.CompressCss)
}

var eventTime = make(map[string]int64)

func configWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic("Failed start app watcher: " + err.Error())
	}

	go func() {
		for {
			select {
			case event := <-watcher.Event:
				switch filepath.Ext(event.Name) {
				case ".ini":
					if checkEventTime(event.Name) {
						continue
					}
					beego.Info(event)

					if err := Cfg.Reload(); err != nil {
						beego.Error("Conf Reload: ", err)
					}

					if err := i18n.ReloadLangs(); err != nil {
						beego.Error("Conf Reload: ", err)
					}

					reloadConfig()
					beego.Info("Config Reloaded")

				case ".json":
					if checkEventTime(event.Name) {
						continue
					}
					if event.Name == CompressConfPath {
						settingCompress()
						beego.Info("Beego Compress Reloaded")
					}
				}
			}
		}
	}()

	if err := watcher.WatchFlags("conf", fsnotify.FSN_MODIFY); err != nil {
		beego.Error(err)
	}
}

// checkEventTime returns true if FileModTime does not change.
func checkEventTime(name string) bool {
	mt := getFileModTime(name)
	if eventTime[name] == mt {
		return true
	}

	eventTime[name] = mt
	return false
}

// getFileModTime retuens unix timestamp of `os.File.ModTime` by given path.
func getFileModTime(path string) int64 {
	path = strings.Replace(path, "\\", "/", -1)
	f, err := os.Open(path)
	if err != nil {
		beego.Error("Fail to open file[ %s ]\n", err)
		return time.Now().Unix()
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		beego.Error("Fail to get file information[ %s ]\n", err)
		return time.Now().Unix()
	}

	return fi.ModTime().Unix()
}

func IsMatchHost(uri string) bool {
	if len(uri) == 0 {
		return false
	}

	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return false
	}

	if u.Host != AppHost {
		return false
	}

	return true
}
