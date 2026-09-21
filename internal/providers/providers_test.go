package providers

import (
	"context"
	"testing"
)

func TestMock(t *testing.T) {
	m := MockProvider{}
	v, e := m.Complete(context.Background(), Request{Prompt: "hello"})
	if e != nil || v != "MOCK: hello" {
		t.Fatalf("%q %v", v, e)
	}
	ch, er := m.Stream(context.Background(), Request{Prompt: "a b"})
	var got int
	for range ch {
		got++
	}
	if e := <-er; e != nil {
		t.Fatal(e)
	}
	if got != 3 {
		t.Fatal(got)
	}
}
