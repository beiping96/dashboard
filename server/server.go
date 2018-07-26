package server

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/sony/sonyflake"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
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
	result := mailPacker(c.FormValue("serverID"), c.FormValue("dbName"),
		c.FormValue("goods"), c.FormValue("title"),
		c.FormValue("content"), c.FormValue("accNames"))
	return c.Render(http.StatusOK, "index", result)
}

func mailPacker(serverIDStr, dbName, goodsStr, title, content, accNames string) string {
	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		return "Server ID is not integer"
	}
	if len(dbName) == 0 {
		return "dbName is empty"
	}

	conn, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8",
		config.DBUser, config.DBPw, config.DBAddr, config.DBPort, dbName))
	if err != nil {
		return "db connection error " + err.Error()
	}
	defer conn.Close()
	if accNames == "" {
		return "empty accnames"
	}

	playerIDs := []int{}
	for _, accname := range strings.Split(accNames, "/") {
		if accname == "" {
			continue
		}
		row := conn.QueryRow("SELECT id FROM player WHERE accname=?", accname)
		playerID := 0
		err = row.Scan(&playerID)
		if err != nil {
			return "accname not found " + accNames + " one: " + accname
		}
		playerIDs = append(playerIDs, playerID)
	}
	if len(playerIDs) == 0 {
		return "not found playerIDs " + accNames
	}

	goods := []string{}
	for _, goodsOne := range strings.Split(goodsStr, "/") {
		if goodsOne == "" {
			continue
		}
		goodsKV := strings.Split(goodsOne, ",")
		if len(goodsKV) != 2 {
			return "check goods List " + goodsStr
		}
		goodsID, err := strconv.Atoi(goodsKV[0])
		if err != nil {
			return "check goods List " + goodsStr
		}
		goodsNum, err := strconv.Atoi(goodsKV[1])
		if err != nil {
			return "check goods List " + goodsStr
		}
		goodsOneStr := fmt.Sprintf("{%d,%d}", goodsID, goodsNum)
		goods = append(goods, goodsOneStr)
	}
	goodsStr = "[" + strings.Join(goods, ",") + "]"
	successNum := 0
	mailIDs := mailIDGenerator(serverID, len(playerIDs))

	for index, playerID := range playerIDs {
		mailID := mailIDs[index]
		_, err := conn.Exec("INSERT INTO player_mail (id, type, send_role_id, rec_role_id, title, content, accessory, ctime) VALUES (?, 1, 0, ?, ?, ?, ?, ?)",
			mailID, playerID, title, content, goodsStr, time.Now().Unix())
		if err != nil {
			return err.Error()
		}
		successNum++
	}

	fmt.Println(serverID, dbName, goods, title, content, accNames)
	return fmt.Sprintf("success %v", successNum)
}

func mailIDGenerator(serverID int, num int) []uint64 {
	generator := sonyflake.NewSonyflake(sonyflake.Settings{MachineID: func() (uint16, error) { return uint16(serverID), nil }})
	ans := make([]uint64, num)
	for index := range ans {
		ans[index], _ = generator.NextID()
	}
	return ans
}
