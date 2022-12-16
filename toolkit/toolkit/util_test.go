package toolkit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPathExistsReturnsCorrectBoolForPath(t *testing.T) {
	tmpdir := t.TempDir()
	if !PathExists(tmpdir) {
		t.Fail()
	}
	if PathExists(filepath.Join(tmpdir, "idontexist")) {
		t.Fail()
	}
}

func TestPathIsDirReturnsCorrectBoolForPath(t *testing.T) {
	tmpdir := t.TempDir()
	if !PathIsDir(tmpdir) {
		t.Fail()
	}
	if PathIsDir(tmpdir + "idontexist") {
		t.Fail()
	}
	myfilePath := filepath.Join(tmpdir, "myfile")
	os.WriteFile(myfilePath, []byte("content"), 0644)
	if PathIsDir(myfilePath) {
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

func TestGetFunctionName(t *testing.T) {
	myCoolFunction := func() {}
	if res, err := GetFunctionName(myCoolFunction); res != "myCoolFunction" && err != nil {
		t.Fail()
	}
	type myStruct struct{}
	if _, err := GetFunctionName(myStruct{}); err == nil {
		t.Fail()
	}
}

func TestPullStrKey(t *testing.T) {
	testInput := map[string]any{
		"key1": "val1",
		"key2": 0,
	}
	if v, err := PullStrKey("key1", testInput); v != "val1" && err != nil {
		t.Fail()
	}
	if _, err := PullStrKey("key2", testInput); err == nil {
		t.Fail()
	}
	if _, err := PullStrKey("key3", testInput); err == nil {
		t.Fail()
	}
	if _, err := PullStrKey("key4", map[string]any{}); err == nil {
		t.Fail()
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

func TestSliceContains(t *testing.T) {
	if SliceContains([]int{}, 0) {
		t.Fail()
	}
	if SliceContains([]int{1, 2, 3}, 0) {
		t.Fail()
	}
	if !SliceContains([]string{"a", "b", "c"}, "a") {
		t.Fail()
	}
	if !SliceContains([]string{"a", "b", "c"}, "c") {
		t.Fail()
	}
}

func TestMakeRange(t *testing.T) {
	myRange := MakeRange(0, 10, 2)
	if len(myRange) != 5 {
		t.FailNow()
	}
	if myRange[0] != 0 {
		t.Fail()
	}
	if myRange[4] != 8 {
		t.Fail()
	}
}
