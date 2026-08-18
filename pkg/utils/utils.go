package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/netip"
	"os"
	"regexp"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

var (
	Json = jsoniter.ConfigCompatibleWithStandardLibrary

	DNSServers = []string{"1.1.1.1:53", "223.5.5.5:53"}
)

func IsWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

var ipv4Re = regexp.MustCompile(`(\d*\.).*(\.\d*)`)

// ipv4Desensitize 保留 A.B、隐藏 C.D；切不出 4 段时回退正则打码兜底
func ipv4Desensitize(ipv4Addr string) string {
	parts := strings.Split(ipv4Addr, ".")
	if len(parts) != 4 {
		return ipv4Re.ReplaceAllString(ipv4Addr, "$1****$2")
	}
	return parts[0] + "." + parts[1] + ".*.*"
}

var ipv6Re = regexp.MustCompile(`(\w*:\w*:).*(:\w*:\w*)`)

// ipv6Desensitize 保留前 3 个 hextet（/48）；解析失败回退正则打码兜底
func ipv6Desensitize(ipv6Addr string) string {
	addr, err := netip.ParseAddr(ipv6Addr)
	if err != nil {
		return ipv6Re.ReplaceAllString(ipv6Addr, "$1****$2")
	}
	// netip 无导出的 WithoutZone，WithZone("") 等价去 zone
	addr = addr.WithZone("")
	if addr.Is4() || addr.Is4In6() {
		return ipv4Desensitize(addr.Unmap().String())
	}
	b := addr.As16()
	return fmt.Sprintf("%x:%x:%x:****::",
		uint16(b[0])<<8|uint16(b[1]),
		uint16(b[2])<<8|uint16(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
	)
}

// IPDesensitize 按 / 拆双栈串逐段打码后拼回；空段与空串原样保留
func IPDesensitize(ipAddr string) string {
	if ipAddr == "" {
		return ""
	}
	ipList := strings.Split(ipAddr, "/")
	for i, ip := range ipList {
		if ip == "" {
			continue
		}
		if strings.Contains(ip, ":") {
			ipList[i] = ipv6Desensitize(ip)
		} else {
			ipList[i] = ipv4Desensitize(ip)
		}
	}
	return strings.Join(ipList, "/")
}

// SplitIPAddr 传入/分割的v4v6混合地址，返回v4和v6地址与有效地址
func SplitIPAddr(v4v6Bundle string) (string, string, string) {
	ipList := strings.Split(v4v6Bundle, "/")
	ipv4 := ""
	ipv6 := ""
	validIP := ""
	if len(ipList) > 1 {
		// 双栈
		ipv4 = ipList[0]
		ipv6 = ipList[1]
		validIP = ipv4
	} else if len(ipList) == 1 {
		// 仅ipv4|ipv6
		if strings.Contains(ipList[0], ":") {
			ipv6 = ipList[0]
			validIP = ipv6
		} else {
			ipv4 = ipList[0]
			validIP = ipv4
		}
	}
	return ipv4, ipv6, validIP
}

func IsFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GenerateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	lettersLength := big.NewInt(int64(len(letters)))
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, lettersLength)
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}
	return string(ret), nil
}

func Uint64SubInt64(a uint64, b int64) uint64 {
	if b < 0 {
		return a + uint64(-b)
	}
	if a < uint64(b) {
		return 0
	}
	return a - uint64(b)
}
