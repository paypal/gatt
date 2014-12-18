package gatt

import (
	"errors"
	"sync"
)

type notifier struct {
	conn   *conn
	char   *Characteristic
	maxlen int
	donemu sync.RWMutex
	done   bool
}

func newNotifier(c *conn, cc *Characteristic, maxlen int) *notifier {
	return &notifier{conn: c, char: cc, maxlen: maxlen}
}

func (n *notifier) Write(data []byte) (int, error) {
	if n.Done() {
		return 0, errors.New("central stopped notifications")
	}
	return n.conn.sendNotification(n.char, data)
}

func (n *notifier) Cap() int {
	return n.maxlen
}

func (n *notifier) Done() bool {
	n.donemu.RLock()
	done := n.done
	n.donemu.RUnlock()
	return done
}

func (n *notifier) stop() {
	n.donemu.Lock()
	n.done = true
	n.donemu.Unlock()
}
