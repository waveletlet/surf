package util

import (
	"github.com/lostinblue/ut"
	"testing"
)

func TestFileExists(t *testing.T) {
	ut.Run(t)

	ex := FileExists("./util_test.go")
	ut.AssertTrue(ex)

	ex = FileExists("./util.txt")
	ut.AssertFalse(ex)
}
