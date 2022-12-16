package toolkit

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/constraints"
)

// ExitWithError will call os.Exit(1) if the given err is not nil, logging that something bad happened.
func ExitWithError(logger *log.Logger, err error) {
	logger.WithField("err", err).Error("❗something really bad happened, unable to continue")
	os.Exit(1)
}

// PathExists return true if the given path exists on disk.
// Simple wrapper around os.Stat.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// PathIsDir returns if the given path is a directory.
func PathIsDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

// PPrint takes the given value, marshalls it using json and prints it using fmt.
func PPrint(n interface{}) error {
	marshalled, err := json.MarshalIndent(n, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(marshalled))
	return nil
}

// RandomString returns a string of random characters of length n.
func RandomString(n int) string {
	sb := strings.Builder{}
	for n > 0 {
		index := rand.Intn(26)
		sb.WriteString(fmt.Sprintf("%c", 97+index))
		n--
	}
	return sb.String()
}

// GetFunctionName takes in a function and uses reflect to get its name as a string.
func GetFunctionName(i any) (string, error) {
	val := reflect.ValueOf(i)
	kind := val.Kind()
	if kind != reflect.Func {
		return "", fmt.Errorf("expected a function, instead got: %s", kind.String())
	}
	funcPointer := runtime.FuncForPC(val.Pointer())
	if funcPointer == nil {
		return "", fmt.Errorf("unable to get func pointer for given function")
	}
	return funcPointer.Name(), nil
}

// PullStrKey returns the string value of the given key within the given string to interface map.
// Useful when working with unstructured kubernetes objects.
func PullStrKey(k string, m map[string]interface{}) (string, error) {
	_v, ok := m[k]
	if !ok {
		return "", fmt.Errorf("could not find key %s in map: %s", k, m)
	}
	v, ok := _v.(string)
	if !ok {
		return "", fmt.Errorf("could not turn key %s in map into string; %s", k, m)
	}
	return v, nil
}

// SimpleRetry will run the target function maxAttempts times, waiting delay in-between attempts.
// If a logger is given, then attempts will be logged.
func SimpleRetry(target func() error, maxAttempts int, delay time.Duration, logger ...*log.Logger) error {
	attempts := 0
	funcName, err := GetFunctionName(target)
	if err != nil {
		return err
	}

	doLog := func(msg string, err ...error) {
		if len(logger) > 0 {
			for _, l := range logger {
				lf := l.WithFields(log.Fields{
					"name":         funcName,
					"max-attempts": maxAttempts,
					"delay":        delay.String(),
				})
				if len(err) > 0 {
					for _, e := range err {
						lf = lf.WithError(e)
					}
				}
				lf.Debug(msg)
			}
		}
	}

	doLog("running function until success")

	for attempts < maxAttempts {
		doLog(fmt.Sprintf("attempt %d", attempts))
		err = target()
		if err == nil {
			doLog(fmt.Sprintf("success on attempt %d", attempts))
			return nil
		}
		doLog("failed, sleeping", err)
		time.Sleep(delay)
		attempts++
	}

	return err
}

// SliceContains is a generic that checks if the given slices contains the given target.
func SliceContains[T comparable](slice []T, target T) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// MakeRange is a generic that creates a slice of numbers in the range
// [min, max), separated by the given step value.
func MakeRange[T interface {
	constraints.Integer | constraints.Float
}](min, max, step T) []T {
	a := []T{}
	i := min
	for i < max {
		a = append(a, i)
		i += step
	}
	return a
}
