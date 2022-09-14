package toolkit

import (
	"errors"
	"testing"
	"time"
)

func TestPathExistsReturnsCorrectBoolForPath(t *testing.T) {
	tmpdir := t.TempDir()
	if !PathExists(tmpdir) {
		t.Fail()
	}
	if PathExists(tmpdir + "idontexist") {
		t.Fail()
	}
}

func TestRandomStringUsesWholeAlphabet(t *testing.T) {
	containsFalse := func(slc []bool) bool {
		for _, b := range slc {
			if !b {
				return false
			}
		}
		return true
	}

	alpha := make([]bool, 26)
	alpha[0] = false

	maxLoops := 1
	var randomStr string
	for !containsFalse(alpha) && maxLoops > 0 {
		randomStr = RandomString(26)
		for _, c := range randomStr {
			alpha[int(c)-97] = true
		}
		maxLoops--
	}

	if !containsFalse(alpha) {
		return
	}
	t.Fail()
}

func TestRandomStringRespectsGivenLength(t *testing.T) {
	i := 0
	var randomStr string
	for i < 100 {
		randomStr = RandomString(i)
		if len(randomStr) != i {
			t.Errorf("expected str of len %d", i)
			t.FailNow()
		}
		i++
	}
}

func TestSimpleRetryRespectsMaxAttempts(t *testing.T) {
	var counter *int = new(int)
	*counter = 0

	start := time.Now()
	SimpleRetry(
		func() error {
			*counter += 1
			return errors.New("my-fake-error")
		}, 5, 100*time.Millisecond,
	)
	end := time.Now()

	if *counter != 5 {
		t.Errorf("did not retry 5 times: %d", *counter)
		t.FailNow()
	}

	duration := end.Sub(start)
	if duration < 5*100*time.Millisecond {
		t.Errorf("duration is less than 500 milliseconds: %s", duration)
		t.FailNow()
	}
	if duration > 6*100*time.Millisecond {
		t.Errorf("duration is greater than 600 milliseconds: %s", duration)
		t.FailNow()
	}
}
