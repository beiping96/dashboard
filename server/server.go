package server

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

var (
	config *Config
)

// Config desc cardWS.xml
type Config struct {
	ListenPort string `xml:"listen_port"`
	LogLevel   string `xml:"log_level"`
	DBAddr     string `xml:"db_addr"`
	DBPort     string `xml:"db_port"`
	DBUser     string `xml:"db_user"`
	DBPw       string `xml:"db_pw"`
	User       string `xml:"user"`
	PassWord   string `xml:"pw"`
}

// DashboardRenderer define for dashboard template renderer
type DashboardRenderer struct {
	templates *template.Template
}

// Render achieve DashboardRenderer method render
func (t *DashboardRenderer) Render(w io.Writer, name string,
	data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// Start echo server
func Start(configFile string) error {
	// config
	configSource, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("read config file error file:%v",
			configFile)
	}
	config = &Config{}
	err = xml.Unmarshal(configSource, config)
	if err != nil {
		return fmt.Errorf("unmarshal config file error file:%v, error:%v",
			configFile, err)
	}
	// echo
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.Logger())
	switch config.LogLevel {
	case "debug":
		e.Logger.SetLevel(log.DEBUG)
	case "info":
		e.Logger.SetLevel(log.INFO)
	case "warn":
		e.Logger.SetLevel(log.WARN)
	default:
		e.Logger.SetLevel(log.ERROR)
	}
	e.Static("/", "./static")
	renderer := &DashboardRenderer{
		templates: template.Must(template.ParseGlob("./static/*.html")),
	}
	e.Renderer = renderer
	e.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
		Skipper:   authSkipper,
		Validator: authValidator,
	}))
	e.GET("/", index)
	e.GET("/mailSender", index)
	e.POST("/mailSender", mailSender)
	err = e.Start(config.ListenPort)
	if err != nil {
		return fmt.Errorf("server %v start error %v",
			config.ListenPort, err)
	}
	return nil
}

func authSkipper(c echo.Context) bool {
	cookie, err := c.Cookie("auth")
	if err != nil {
		return false
	}
	if cookie.Value == generateCookie(config.User, config.PassWord) {
		return true
	}
	return false
}

func authValidator(user string, passWord string, c echo.Context) (bool, error) {
	if user != config.User || passWord != config.PassWord {
		return false, nil
	}
	cookie := new(http.Cookie)
	cookie.Name = "auth"
	cookie.Value = generateCookie(user, passWord)
	cookie.Expires = time.Now().Add(time.Hour * 2)
	cookie.HttpOnly = true
	fmt.Printf("cookie expire: %d", cookie.Expires.Unix())
	c.SetCookie(cookie)
	return true, nil
}

func generateCookie(user string, passWord string) string {
	md5ctx := md5.New()
	md5ctx.Write([]byte(user + passWord))
	salt := strconv.FormatInt(getExpireTimestamp(), 10)
	salt += "salt SS$*sd^&(b^er$-1fw(da)d"
	md5ctx.Write([]byte(salt))
	res := md5ctx.Sum(nil)
	return hex.EncodeToString(res)
}

func getExpireTimestamp() int64 {
	expireTime := []uint32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22}
	now := time.Now()
	hour := uint32(now.Hour())
	expire := uint32(0)
	for i := len(expireTime) - 1; i >= 0; i-- {
		if hour < expireTime[i] {
			continue
		}
		expire = expireTime[i] + 2
		if expire >= 24 {
			expire = 24
		}
		break
	}
	timeZero := time.Date(now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0, now.Location())
	timeHour := timeZero.Add(time.Hour * time.Duration(expire))
	return timeHour.Unix()
}

func index(c echo.Context) error {
	return c.Render(http.StatusOK, "index", nil)
}

func mailSender(c echo.Context) error {
	return nil
}
