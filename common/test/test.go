package test

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"testing"
	"unicode/utf8"
)

func FailOnError(t testing.TB, err error) {
	if err == nil {
		return
	}
	t.Errorf("[ERROR] %v", err)
	debug.PrintStack()
	t.FailNow()
}

func Equals(t testing.TB, exp interface{}, act interface{}, format string, args ...interface{}) {
	if reflect.DeepEqual(exp, act) {
		return
	}
	t.Errorf("[ERROR] %v. exp: %+v; act: %+v", fmt.Sprintf(format, args...), exp, act)
	expBytes, expIsBytes := exp.([]byte)
	actBytes, actIsBytes := act.([]byte)

	if expIsBytes && actIsBytes && utf8.Valid(expBytes) && utf8.Valid(actBytes) {
		t.Errorf("exp: %v", string(expBytes))
		t.Errorf("act: %v", string(actBytes))
	}
	debug.PrintStack()
	t.FailNow()
}
