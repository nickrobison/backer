package daemon

// Borrowed heavily from: github.com/cespare/reflex

// Backlog - struct for maintaining a list of events to process
type Backlog interface {
	Add(path BackerEvent)
	Next() BackerEvent
	RemoveOne() (empty bool)
}

// MultiFileBacklog - data structure for maintaining Backlog state
type MultiFileBacklog struct {
	empty bool
	next  BackerEvent
	rest  map[BackerEvent]struct{}
}

// NewMultiFileBacklog - Creates a new Backlog
func NewMultiFileBacklog() *MultiFileBacklog {
	return &MultiFileBacklog{
		empty: true,
		next:  BackerEvent{},
		rest:  make(map[BackerEvent]struct{}),
	}
}

// Add - Add a file event to the backlog
func (b *MultiFileBacklog) Add(event BackerEvent) {
	defer func() {
		b.empty = false
	}()
	if b.empty {
		b.next = event
		return
	}
	if b.next.equals(event) {
		return
	}
	b.rest[event] = struct{}{}
}

// Next - retrieves the next BackerEvent from the Backlog
func (b *MultiFileBacklog) Next() BackerEvent {
	if b.empty {
		logger.Fatalln("Empty backlog, can't get next")
	}
	return b.next
}

// RemoveOne - Removes a single BackerEvent from the Backlog
func (b *MultiFileBacklog) RemoveOne() bool {
	if b.empty {
		logger.Fatalln("Empty backlog, can't remove")
	}
	if len(b.rest) == 0 {
		b.next = BackerEvent{}
		b.empty = true
		return true
	}
	for next := range b.rest {
		b.next = next
		break
	}
	delete(b.rest, b.next)
	return false
}
