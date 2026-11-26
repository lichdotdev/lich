package winretry

import (
	"errors"
	"testing"
)

func TestDoAnswersWhatOpAnswers(t *testing.T) {
	if err := Do(func() error { return nil }); err != nil {
		t.Fatalf("Do = %v, want nil", err)
	}
	want := errors.New("no")
	if err := Do(func() error { return want }); !errors.Is(err, want) {
		t.Fatalf("Do = %v, want %v", err, want)
	}
}
