package entropy

import "testing"

func TestWindow_AllLowEntropy(t *testing.T) {
	w := NewWindow(WindowConfig{
		Size:           5,
		Threshold:      1.5,
		EarlyExitCount: 3,
	})

	for i := 0; i < 20; i++ {
		d := w.Add(0.3)
		if d != Continue {
			t.Fatalf("token %d: expected Continue, got Escalate", i)
		}
	}
}

func TestWindow_HighEntropyExceedsThreshold(t *testing.T) {
	w := NewWindow(WindowConfig{
		Size:           5,
		Threshold:      1.5,
		EarlyExitCount: 3,
	})

	var escalated bool
	for i := 0; i < 10; i++ {
		d := w.Add(2.0)
		if d == Escalate {
			escalated = true
			break
		}
	}

	if !escalated {
		t.Error("expected Escalate with high entropy values")
	}
}

func TestWindow_EarlyExit(t *testing.T) {
	w := NewWindow(WindowConfig{
		Size:           10,
		Threshold:      1.5,
		EarlyExitCount: 5,
	})

	d := w.Add(2.5)
	if d != Escalate {
		t.Error("expected Escalate on high-entropy token during early exit period")
	}
}

func TestWindow_EarlyExitNotTriggeredAfterPeriod(t *testing.T) {
	w := NewWindow(WindowConfig{
		Size:           5,
		Threshold:      1.5,
		EarlyExitCount: 3,
	})

	for i := 0; i < 3; i++ {
		d := w.Add(0.5)
		if d != Continue {
			t.Fatalf("token %d: expected Continue", i)
		}
	}

	d := w.Add(2.0)
	if d != Continue {
		t.Error("expected Continue: past early exit period and window not full yet")
	}
}

func TestWindow_Recovery(t *testing.T) {
	w := NewWindow(WindowConfig{
		Size:           5,
		Threshold:      1.5,
		EarlyExitCount: 0,
	})

	for i := 0; i < 4; i++ {
		w.Add(2.0)
	}

	for i := 0; i < 5; i++ {
		w.Add(0.1)
	}

	d := w.Add(0.1)
	if d != Continue {
		t.Error("expected Continue after recovery")
	}
	if avg := w.Average(); avg > 1.5 {
		t.Errorf("average should be low after recovery, got %f", avg)
	}
}

func TestWindow_Average(t *testing.T) {
	w := NewWindow(WindowConfig{
		Size:           3,
		Threshold:      10,
		EarlyExitCount: 0,
	})

	w.Add(1.0)
	w.Add(2.0)
	w.Add(3.0)

	got := w.Average()
	want := 2.0
	if got != want {
		t.Errorf("average: got %f, want %f", got, want)
	}

	w.Add(6.0)
	got = w.Average()
	want = (6.0 + 2.0 + 3.0) / 3.0
	if got != want {
		t.Errorf("average after eviction: got %f, want %f", got, want)
	}
}

func TestWindow_Reset(t *testing.T) {
	w := NewWindow(WindowConfig{
		Size:           5,
		Threshold:      1.5,
		EarlyExitCount: 3,
	})

	w.Add(2.0)
	w.Add(2.0)

	w.Reset()

	if w.Average() != 0 {
		t.Errorf("average after reset: got %f, want 0", w.Average())
	}

	d := w.Add(0.5)
	if d != Continue {
		t.Error("expected Continue after reset with low entropy")
	}
}
