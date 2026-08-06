package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	log "github.com/shyunku-libraries/go-logger"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

func CheckEnvironmentVariables(checkKeys []string) error {
	if err := godotenv.Load(); err != nil {
		log.Error(err)
		return err
	}

	missingVariables := make([]string, 0)
	for _, key := range checkKeys {
		if os.Getenv(key) == "" {
			missingVariables = append(missingVariables, key)
		}

		//log.Debugf("env[%s]: %s", key, os.Getenv(key))
	}

	if len(missingVariables) > 0 {
		missingVarKeys := strings.Join(missingVariables, ", ")
		log.Error("Missing environment variables: ", missingVarKeys)
		return errors.New("missing environment variables: " + missingVarKeys)
	}

	return nil
}

func GetProjectRootDirectory() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(b))
}

func ParseDuration(str string) (time.Duration, error) {
	// Duration string without last character (the unit)
	valueStr := str[:len(str)-1]

	// Parse the duration value as a float64
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration string: %v", str)
	}

	// Get the duration unit (last character)
	unit := str[len(str)-1:]

	// Convert the duration value to a time.Duration based on the unit
	switch unit {
	case "c": // century
		return time.Duration(value * float64(time.Hour) * 24 * 365 * 100), nil
	case "y": // year
		return time.Duration(value * float64(time.Hour) * 24 * 365), nil
	case "w": // week
		return time.Duration(value * float64(time.Hour) * 24 * 7), nil
	case "d": // day
		return time.Duration(value * float64(time.Hour) * 24), nil
	case "h": // hour
		return time.Duration(value * float64(time.Hour)), nil
	case "m": // minute
		return time.Duration(value * float64(time.Minute)), nil
	case "s": // second
		return time.Duration(value * float64(time.Second)), nil
	case "ms": // millisecond
		return time.Duration(value * float64(time.Millisecond)), nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %v", unit)
	}
}

func GetPublicIp() (string, error) {
	resp, err := http.Get("http://ipinfo.io/ip")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ipv4 := strings.TrimSpace(string(ip))

	// check ipv4 with regex
	ipv4_regex := `^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`
	if match, _ := regexp.MatchString(ipv4_regex, ipv4); !match {
		return "", fmt.Errorf("invalid ipv4: %v", ipv4)
	}
	return ipv4, nil
}

func StdFormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func StructToReadable(src interface{}) *bytes.Buffer {
	jsonData, err := json.Marshal(src)
	if err != nil {
		log.Error(err)
		return nil
	}
	buffer := bytes.NewBuffer(jsonData)
	return buffer
}

func Sha256(data string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}

func Factorial(n int64) int64 {
	if n == 0 {
		return 1
	}
	return n * Factorial(n-1)
}

func Permutation(n int64, r int64) int64 {
	return Factorial(n) / Factorial(n-r)
}

func MemorySizeOfArray[T any](obj []T) string {
	if len(obj) == 0 {
		return "0 bytes"
	}
	size := float64(unsafe.Sizeof(obj) * unsafe.Sizeof(obj[0]) * uintptr(len(obj)))
	if size < 1024 {
		return fmt.Sprintf("%.f bytes", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.3f KB", size/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.3f MB", size/1024/1024)
	} else {
		return fmt.Sprintf("%.3f GB", size/1024/1024/1024)
	}
}
