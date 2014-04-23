CC=gcc
DESTDIR=/usr/local/bin

all:
	$(CC) hci-ble.c -lbluetooth -o hci-ble
	$(CC) l2cap-ble.c -lbluetooth -o l2cap-ble

install:
	mv hci-ble $(DESTDIR)
	mv l2cap-ble $(DESTDIR)
	setcap 'CAP_NET_RAW=+ep' $(DESTDIR)/hci-ble

clean:
	rm -f hci-ble l2cap-ble
