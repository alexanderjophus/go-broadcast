package broadcast

type taggedObservation[T any] struct {
	sub *subObserver[T]
	ob  T
}

const (
	register = iota
	unregister
	purge
)

type taggedRegReq[T any] struct {
	sub     *subObserver[T]
	ch      chan<- T
	regType int
}

// A MuxObserver multiplexes several streams of observations onto a
// single delivery goroutine.
type MuxObserver[T any] struct {
	subs  map[*subObserver[T]]map[chan<- T]bool
	reg   chan taggedRegReq[T]
	input chan taggedObservation[T]
}

// NewMuxObserver constructs  a new MuxObserver.
//
// qlen is the size of the channel buffer for observations sent into
// the mux observer and reglen is the size of the channel buffer for
// registration/unregistration events.
func NewMuxObserver[T any](qlen, reglen int) *MuxObserver[T] {
	rv := &MuxObserver[T]{
		subs:  map[*subObserver[T]]map[chan<- T]bool{},
		reg:   make(chan taggedRegReq[T], reglen),
		input: make(chan taggedObservation[T], qlen),
	}
	go rv.run()
	return rv
}

// Close shuts down this mux observer.
func (m *MuxObserver[T]) Close() error {
	close(m.reg)
	return nil
}

func (m *MuxObserver[T]) broadcast(to taggedObservation[T]) {
	for ch := range m.subs[to.sub] {
		ch <- to.ob
	}
}

func (m *MuxObserver[T]) doReg(tr taggedRegReq[T]) {
	mm, exists := m.subs[tr.sub]
	if !exists {
		mm = map[chan<- T]bool{}
		m.subs[tr.sub] = mm
	}
	mm[tr.ch] = true
}

func (m *MuxObserver[T]) doUnreg(tr taggedRegReq[T]) {
	mm, exists := m.subs[tr.sub]
	if exists {
		delete(mm, tr.ch)
		if len(mm) == 0 {
			delete(m.subs, tr.sub)
		}
	}
}

func (m *MuxObserver[T]) handleReg(tr taggedRegReq[T]) {
	switch tr.regType {
	case register:
		m.doReg(tr)
	case unregister:
		m.doUnreg(tr)
	case purge:
		delete(m.subs, tr.sub)
	}
}

func (m *MuxObserver[T]) run() {
	for {
		select {
		case tr, ok := <-m.reg:
			if ok {
				m.handleReg(tr)
			} else {
				return
			}
		default:
			select {
			case to := <-m.input:
				m.broadcast(to)
			case tr, ok := <-m.reg:
				if ok {
					m.handleReg(tr)
				} else {
					return
				}
			}
		}
	}
}

// Sub creates a new sub-broadcaster from this MuxObserver.
func (m *MuxObserver[T]) Sub() Broadcaster[T] {
	return &subObserver[T]{m}
}

type subObserver[T any] struct {
	mo *MuxObserver[T]
}

func (s *subObserver[T]) Register(ch chan<- T) {
	s.mo.reg <- taggedRegReq[T]{s, ch, register}
}

func (s *subObserver[T]) Unregister(ch chan<- T) {
	s.mo.reg <- taggedRegReq[T]{s, ch, unregister}
}

func (s *subObserver[T]) Close() error {
	s.mo.reg <- taggedRegReq[T]{s, nil, purge}
	return nil
}

func (s *subObserver[T]) Submit(ob T) {
	s.mo.input <- taggedObservation[T]{s, ob}
}

func (s *subObserver[T]) TrySubmit(ob T) bool {
	if s == nil {
		return false
	}
	select {
	case s.mo.input <- taggedObservation[T]{s, ob}:
		return true
	default:
		return false
	}
}
