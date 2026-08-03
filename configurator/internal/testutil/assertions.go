package testutil

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

// AssertEqual проверяет равенство двух значений.
func AssertEqual[T any](t testing.TB, got, want T, msg string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got %v (type %T), want %v (type %T)", msg, got, got, want, want)
	}
}

// AssertTrue проверяет, что условие истинно.
func AssertTrue(t testing.TB, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatalf("%s: expected true, got false", msg)
	}
}

// AssertFalse проверяет, что условие ложно.
func AssertFalse(t testing.TB, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Fatalf("%s: expected false, got true", msg)
	}
}

// AssertNil проверяет, что значение равна nil или ошибки нет.
func AssertNil(t testing.TB, val any, msg string) {
	t.Helper()
	if val == nil {
		return
	}
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		if v.IsNil() {
			return
		}
	}
	t.Fatalf("%s: expected nil, got %v", msg, val)
}

// AssertNotNil проверяет, что значение не равно nil.
func AssertNotNil(t testing.TB, val any, msg string) {
	t.Helper()
	if val == nil {
		t.Fatalf("%s: expected non-nil, got nil", msg)
	}
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		if v.IsNil() {
			t.Fatalf("%s: expected non-nil, got nil", msg)
		}
	}
}

// AssertErrorIs проверяет наличие конкретной ошибки через errors.Is.
func AssertErrorIs(t testing.TB, err, target error, msg string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: expected error %v, got %v", msg, target, err)
	}
}

// AssertInDelta проверяет, что разность чисел не превышает delta.
func AssertInDelta(t testing.TB, got, want, delta float64, msg string) {
	t.Helper()
	if math.Abs(got-want) > delta {
		t.Fatalf("%s: |%f - %f| > delta %f", msg, got, want, delta)
	}
}

// AssertPanics проверяет, что вызов функции вызывает панику.
func AssertPanics(t testing.TB, fn func(), msg string) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("%s: expected panic, but function executed cleanly", msg)
		}
	}()
	fn()
}
