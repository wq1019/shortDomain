package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/julienschmidt/httprouter"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	VAL   = 0x3FFFFFFF
	INDEX = 0x0000003D
)

var (
	dbDriver = "mysql"
	dbConfig = ""
	alphabet = []byte("abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

type Link struct {
	ID        uint   `gorm:"primary_key;AUTO_INCREMENT"`
	Link      string `gorm:"type:text"`
	ShortLink string `gorm:"not null;unique"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func getEnv(key, defaultVal string) string {
	env := os.Getenv(key)
	if env == "" {
		env = defaultVal
	}
	return env
}

func init() {
	dbName := getEnv("DB_DATABASE", "short_link")
	dbUser := getEnv("DB_USERNAME", "root")
	dbPass := getEnv("DB_PASSWORD", "sunlong0717")
	dbHost := getEnv("DB_HOST", "172.19.0.2")

	dbConfig = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":3306)/" + dbName + "?charset=utf8&parseTime=True&loc=Local"
	db, err := gorm.Open(dbDriver, dbConfig)
	if err != nil {
		fmt.Printf("数据库连接失败: %v", err.Error())
		os.Exit(1)
	}
	defer db.Close()
	db.AutoMigrate(Link{})
}

func main() {
	router := httprouter.New()
	router.GET("/t/:link", Rediract)
	router.POST("/short/store", Store)
	router.GET("/", Show)
	if err := http.ListenAndServe(":8000", router); err != nil {
		log.Fatal(err)
	}
}

// 重定向到某个页面
func Rediract(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	l := ps.ByName("link")
	db, err := gorm.Open(dbDriver, dbConfig)
	if err != nil {
		fmt.Fprintf(w, "数据库异常: %v", err.Error())
		os.Exit(1)
	}
	defer db.Close()
	dbLink := Link{}
	if err := db.Where("short_link = ?", l).First(&dbLink).Error; err != nil {
		fmt.Fprintf(w, "无效的短链接")
	} else {

		http.Redirect(w, r, parseUrl(dbLink.Link), 302)
	}
}

// 解析url
func parseUrl(url string) string {
	flag := url[:7]
	if flag == "http://" || flag == "https:/" {
		return url
	}
	return "http://" + url
}

// 显示生成短链接的页面
func Show(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	crutime := time.Now().Unix()
	h := md5.New()

	io.WriteString(h, strconv.FormatInt(crutime, 10))
	token := fmt.Sprintf("%x", h.Sum(nil))
	t, _ := template.ParseFiles("index.html")
	t.Execute(w, token)
}

// 保存短链接
func Store(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	r.ParseForm()

	originUrl := r.Form.Get("link")
	if checkToken(r) == false {
		fmt.Fprintf(w, "Token 验证失败")
		os.Exit(1)
	}
	db, err := gorm.Open(dbDriver, dbConfig)
	if err != nil {
		fmt.Fprintf(w, "数据库连接失败: %v", err.Error())
		os.Exit(1)
	}
	defer db.Close()
	shortUrl := generageUrl(originUrl)

	// 记录不存在则创建
	if notExist := db.Where("short_link = ?", shortUrl[0]).First(&Link{}).RecordNotFound(); notExist {
		link := new(Link)
		link.Link = originUrl
		link.ShortLink = shortUrl[0]
		db.Create(&link)
	}
	baseHost := getEnv("SHORT_HOST", "http://localhost:8000")
	baseHost += "/t/"
	ResultUrl := baseHost + shortUrl[0]
	t, _ := template.ParseFiles("show.html")
	t.Execute(w, ResultUrl)
}

// 短链接生成：
// 将原长链接进行md5校验和计算，生成32位字符串
// 将32位字符串每8位划分一段，得到4段子串。将每个字串（16进制形式）转化为整型数值，与0x3FFFFFFF按位与运算，生成一个30位的数值
// 将上述生成的30位数值按5位为单位依次提取，得到的数值与0x0000003D按位与，获取一个0-61的整型数值，作为从字符数组中提取字符的索引。得到6个字符就生成了一个短链
// 4段字串共可以生成4个短链
func generageUrl(longUrl string) [4]string {
	longUrlMd5 := Md5(longUrl)
	var result [4]string
	for i := 0; i < 4; i++ {
		tmpUrl := longUrlMd5[i*8 : (i+1)*8]
		calcTmpUrl, _ := strconv.ParseInt(tmpUrl, 16, 64)
		tmpVal := int64(VAL) & calcTmpUrl
		var index int64
		var uri []byte
		for j := 0; j < 6; j++ {
			index = INDEX & tmpVal
			uri = append(uri, alphabet[index])
			tmpVal >>= 5
		}
		result[i] = string(uri)
	}
	return result
}

// 生成指定字符串的 md5
func Md5(str string) string {
	m := md5.New()
	m.Write([]byte(str))
	c := m.Sum(nil)
	return hex.EncodeToString(c)
}

// 这里需要对 Token 进行详细的验证
func checkToken(r *http.Request) bool {
	token := r.Form.Get("token")
	if len(token) < 32 {
		return false
	}
	return true
}
