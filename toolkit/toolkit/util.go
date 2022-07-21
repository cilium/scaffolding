package toolkit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
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

func GetFileSHA256(target string) (string, error) {
	f, err := os.Open(target)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func DownloadFile(dest string, url string, expectedSHA256 string, idempotent ...bool) (int, error) {
	check_exists := true

	if len(idempotent) > 0 {
		if !idempotent[0] {
			check_exists = false
		}
	}

	if check_exists {
		if expectedSHA256 == "" {
			return 0, errors.New("unable to idempotently download file, no sha256 given")
		}
		_, err := os.Stat(dest)
		if err == nil {
			destSHA256, err := GetFileSHA256(dest)
			if err != nil {
				return 0, err
			}
			if destSHA256 == expectedSHA256 {
				return 0, nil
			}
		}
	}

	outFile, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	var resp *http.Response
	err = SimpleRetry(
		func() error {
			resp, err = http.Get(url)
			return err
		},
		3, 5*time.Second,
	)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status from %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if expectedSHA256 != "" {
		h := sha256.New()
		if _, err := h.Write(body); err != nil {
			return 0, err
		}
		resultSHA256 := hex.EncodeToString(h.Sum(nil))
		if resultSHA256 != expectedSHA256 {
			return 0, fmt.Errorf(
				"sha256 mismatch from %s: got: %s, expected: %s",
				url,
				resultSHA256,
				expectedSHA256,
			)
		}
	}

	n, err := outFile.Write(body)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func ExecInPod() {}
