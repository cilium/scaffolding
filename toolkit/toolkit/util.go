package toolkit

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func ExitWithError(logger *log.Logger, err error) {
	logger.WithFields(
		log.Fields{
			"err": err,
		},
	).Error("❗something really bad happened, unable to continue")
	os.Exit(1)
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func PathIsDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

func PPrint(n interface{}) error {
	marshalled, err := json.MarshalIndent(n, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(marshalled))
	return nil
}

func RandomString(n int) string {
	sb := strings.Builder{}
	for n > 0 {
		index := rand.Intn(26)
		sb.WriteString(fmt.Sprintf("%c", 97+index))
		n--
	}
	return sb.String()
}

func SimpleRetry(target func() error, maxAttempts int, delay time.Duration) error {
	attempts := 0
	var err error = nil

	for attempts < maxAttempts {
		err = target()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
		attempts++
	}

	return err
}
