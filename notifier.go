package gatt

import "errors"

type notifier struct {
	conn   *conn
	char   *Characteristic
	maxlen int
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

func (n *notifier) Cap() int   { return n.maxlen }
func (n *notifier) Done() bool { return n.done }
func (n *notifier) stop()      { n.done = true }
