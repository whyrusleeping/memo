package memo

import (
	"testing"
	"time"
)

func TestCacher(t *testing.T) {
	c := NewCacher()

	var called int

	val, err := c.Get("foo", time.Millisecond*50, func() (interface{}, error) {
		called++
		return called, nil
	})

	if err != nil {
		t.Fatal(err)
	}

	if val.(int) != 1 {
		t.Fatal("expected 1")
	}

	// check that value stays cached
	val, err = c.Get("foo", time.Millisecond*50, func() (interface{}, error) {
		called++
		return called, nil
	})

	if err != nil {
		t.Fatal(err)
	}

	if val.(int) != 1 {
		t.Fatal("expected 1")
	}

	time.Sleep(time.Millisecond * 50)

	// check that cached value is returned still immediately after cache expires
	val, err = c.Get("foo", time.Millisecond*50, func() (interface{}, error) {
		called++
		return called, nil
	})

	if err != nil {
		t.Fatal(err)
	}

	if val.(int) != 1 {
		t.Fatal("expected 1")
	}

	time.Sleep(time.Millisecond * 10)

	// check that cached value is updated shortly afterwards
	val, err = c.Get("foo", time.Millisecond*50, func() (interface{}, error) {
		called++
		return called, nil
	})

	if err != nil {
		t.Fatal(err)
	}

	if val.(int) != 2 {
		t.Fatal("expected 2")
	}
}
