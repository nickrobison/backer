// Borrowed heavily from: github.com/cespare/reflex
package main


type Backlog interface {
    Add(path BackerEvent)
    Next() BackerEvent
    RemoveOne() (empty bool)
}

type MultiFileBacklog struct {
    empty bool
    next BackerEvent
    rest map[BackerEvent]struct{}
}

func NewMultiFileBacklog() *MultiFileBacklog {
    return &MultiFileBacklog{
        empty: true,
        next: BackerEvent{},
        rest: make(map[BackerEvent]struct{}),
    }
}

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

func (b *MultiFileBacklog) Next() BackerEvent {
    if b.empty {
        logger.Fatalln("Empty backlog, can't get next")
    }
    return b.next;
}

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