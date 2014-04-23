// Package gatt provides a Bluetooth Low Energy gatt implementation.
//
// Gatt (Generic Attribute Profile) is the protocol used to write
// BLE peripherals (servers) and centrals (clients).
//
// STATUS
//
// This package is a work in progress. The API will change.
//
// Support for writing a peripheral is mostly done: You
// can create services and characteristics, advertise,
// accept connections, and handle requests.
// Central support is missing: Scan, connect, discover services
// and characteristics, make requests.
//
//
// SETUP
//
// gatt only supports Linux, with BlueZ installed. This may change.
//
// Installed the required packages, e.g.:
//
//     sudo apt-get install bluez libbluetooth-dev libcap2-bin
//
// If you have BlueZ 5.14+ (or aren't sure), stop the built-in
// bluetooth server, which interferes with gatt, e.g.:
//
//     sudo service bluetooth stop
//
// gatt uses two helper executables. The source for them is in the
// c directory. There's an included makefile. It currently assumes
// that your native compiler is gcc and that you want the executables
// in /usr/local/bin. If /usr/local/bin is not already in your PATH,
// add it. If you don't like those assumptions, edit the makefile.
// (TODO: Get someone with strong makefile-fu to help me clean this up.)
//
//     export PATH="$PATH:/usr/local/bin"
//     cd $GOPATH/src/github.com/paypal/gatt/c
//     make
//     sudo make install
//
// Root is required in the install phase to give hci-ble permissions
// to administer the network.
//
// Make sure that your BLE device is up:
//
//     sudo hciconfig
//     sudo hciconfig hci0 up  # or whatever hci device you want to use
//
//
// USAGE
//
// Gatt servers are constructed by creating a new server, adding
// services and characteristics, and then starting the server.
//
//     srv := &gatt.Server{Name: "gophergatt"}
//     svc := srv.AddService(gatt.MustParseUUID("09fc95c0-c111-11e3-9904-0002a5d5c51b"))
//
//     // Add a read characteristic that prints how many times it has been read
//     n := 0
//     rchar := svc.AddCharacteristic(gatt.MustParseUUID("11fac9e0-c111-11e3-9246-0002a5d5c51b"))
//     rchar.HandleRead(
//     	gatt.ReadHandlerFunc(
//     		func(resp gatt.ReadResponseWriter, req *gatt.ReadRequest) {
//     			fmt.Fprintf(resp, "count: %d", n)
//     			n++
//     		}),
//     )
//
//     // Add a write characteristic that logs when written to
//     wchar := svc.AddCharacteristic(gatt.MustParseUUID("16fe0d80-c111-11e3-b8c8-0002a5d5c51b"))
//     wchar.HandleWriteFunc(
//     	func(r gatt.Request, data []byte) (status byte) {
//     		log.Println("Wrote:", string(data))
//     		return gatt.StatusSuccess
//     	})
//
//     // Add a notify characteristic that updates once a second
//     nchar := svc.AddCharacteristic(gatt.MustParseUUID("1c927b50-c116-11e3-8a33-0800200c9a66"))
//     nchar.HandleNotifyFunc(
//     	func(r gatt.Request, n gatt.Notifier) {
//     		go func() {
//     			count := 0
//     			for !n.Done() {
//     				fmt.Fprintf(n, "Count: %d", count)
//     				count++
//     				time.Sleep(time.Second)
//     			}
//     		}()
//     	})
//
//     // Start the server
//     log.Fatal(srv.AdvertiseAndServe())
//
//
// See the rest of the docs for other options and finer-grained control.
//
// Note that some BLE central devices, particularly iOS, may aggressively
// cache results from previous connections. If you change your services or
// characteristics, you may need to reboot the other device to pick up the
// changes. This is a common source of confusion and apparent bugs. For an
// OS X central, see http://stackoverflow.com/questions/20553957.
//
//
// REFERENCES
//
// gatt started life as a port of bleno, to which it is indebted:
// https://github.com/sandeepmistry/bleno. If you are having
// problems with gatt, particularly around installation, issues
// filed with bleno might also be helpful references.
//
// To try out your GATT server, it is useful to experiment with a
// generic BLE client. LightBlue is a good choice. It is available
// free for both iOS and OS X.
//
package gatt
