package entropy

type Decision int

const (
	Continue Decision = iota
	Escalate
)

type WindowConfig struct {
	Size           int
	Threshold      float64
	EarlyExitCount int
}

type Window struct {
	cfg     WindowConfig
	buf     []float64
	pos     int
	count   int
	sum     float64
	filled  bool
}

func NewWindow(cfg WindowConfig) *Window {
	return &Window{
		cfg: cfg,
		buf: make([]float64, cfg.Size),
	}
}

func (w *Window) Add(entropy float64) Decision {
	if w.filled {
		w.sum -= w.buf[w.pos]
	}

	w.buf[w.pos] = entropy
	w.sum += entropy
	w.pos = (w.pos + 1) % w.cfg.Size
	w.count++

	if w.count >= w.cfg.Size {
		w.filled = true
	}

	if w.count <= w.cfg.EarlyExitCount && entropy > w.cfg.Threshold {
		return Escalate
	}

	if w.filled && w.Average() > w.cfg.Threshold {
		return Escalate
	}

	return Continue
}

func (w *Window) Average() float64 {
	n := w.count
	if n > w.cfg.Size {
		n = w.cfg.Size
	}
	if n == 0 {
		return 0
	}
	return w.sum / float64(n)
}

func (w *Window) Reset() {
	w.pos = 0
	w.count = 0
	w.sum = 0
	w.filled = false
	for i := range w.buf {
		w.buf[i] = 0
	}
}
