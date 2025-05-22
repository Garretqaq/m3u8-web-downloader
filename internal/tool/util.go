package tool

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 暂存需要清理的临时文件/目录
var tempFiles []string
var tempMutex sync.Mutex

// AddTempFile 添加一个需要在程序退出时清理的临时文件或目录
func AddTempFile(path string) {
	tempMutex.Lock()
	defer tempMutex.Unlock()
	tempFiles = append(tempFiles, path)
}

// Cleanup 清理所有临时文件和目录
func Cleanup() {
	tempMutex.Lock()
	defer tempMutex.Unlock()

	for _, path := range tempFiles {
		// 尝试删除文件或目录，忽略错误
		_ = os.RemoveAll(path)
	}
	tempFiles = nil
}

func CurrentDir(joinPath ...string) (string, error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}
	p := strings.Replace(dir, "\\", "/", -1)
	whole := filepath.Join(joinPath...)
	whole = filepath.Join(p, whole)
	return whole, nil
}

func ResolveURL(u *url.URL, p string) string {
	if strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") {
		return p
	}
	var baseURL string
	if strings.Index(p, "/") == 0 {
		baseURL = u.Scheme + "://" + u.Host
	} else {
		tU := u.String()
		baseURL = tU[0:strings.LastIndex(tU, "/")]
	}
	return baseURL + path.Join("/", p)
}

func DrawProgressBar(prefix string, proportion float32, width int, suffix ...string) {
	pos := int(proportion * float32(width))
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	s := fmt.Sprintf("[%s] [%s] %s%*s %6.2f%% %s",
		timestamp, prefix, strings.Repeat("■", pos), width-pos, "", proportion*100, strings.Join(suffix, ""))
	fmt.Print("\r" + s)
}
