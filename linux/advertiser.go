package linux

import (
	"sync"

	"github.com/paypal/gatt/linux/internal/cmd"
)

type advertiser struct {
	advertisingPacket  []byte
	scanResponsePacket []byte
	manufacturerData   []byte

	advertisingIntervalMin uint16
	advertisingIntervalMax uint16
	advertisingChannelMap  uint8

	serving   bool
	servingmu *sync.RWMutex

	cmd *cmd.Cmd
}

func NewAdvertiser(h *HCI) *advertiser {
	return &advertiser{
		advertisingIntervalMin: 0x0800,
		advertisingIntervalMax: 0x0800,
		advertisingChannelMap:  7,

		servingmu: &sync.RWMutex{},

		cmd: h.c,
	}
}

// SetServing enables or disables advertising.
func (a *advertiser) SetServing(s bool) {
	a.servingmu.Lock()
	defer a.servingmu.Unlock()
	a.serving = s
}

// Serving returns the status of advertising.
func (a *advertiser) Serving() bool {
	a.servingmu.RLock()
	defer a.servingmu.RUnlock()
	return a.serving
}

// Start starts advertising.
func (a *advertiser) Start() error {
	a.SetServing(true)
	return a.cmd.SendAndCheckResp(cmd.LESetAdvertiseEnable{AdvertisingEnable: 1}, []byte{0x00})
}

// Stop stops advertising.
func (a *advertiser) Stop() error {
	a.SetServing(false)
	return a.cmd.SendAndCheckResp(cmd.LESetAdvertiseEnable{AdvertisingEnable: 0}, []byte{0x00})
}

func (a *advertiser) AdvertiseService() error {
	if a.Serving() {
		a.Stop()
		defer a.Start()
	}
	a.servingmu.RLock()
	defer a.servingmu.RUnlock()

	if err := a.cmd.SendAndCheckResp(
		cmd.LESetAdvertisingParameters{
			AdvertisingIntervalMin: a.advertisingIntervalMin,
			AdvertisingIntervalMax: a.advertisingIntervalMax,
			AdvertisingChannelMap:  a.advertisingChannelMap,
		}, []byte{0x00}); err != nil {
		return err
	}

	if len(a.scanResponsePacket) > 0 {
		// Scan response command takes exactly 31 bytes data
		// The length indicating the significant part of the data.
		data := [31]byte{}
		n := copy(data[:31], a.scanResponsePacket)
		if err := a.cmd.SendAndCheckResp(
			cmd.LESetScanResponseData{
				ScanResponseDataLength: uint8(n),
				ScanResponseData:       data,
			}, []byte{0x00}); err != nil {
			return err
		}
	}

	if len(a.advertisingPacket) > 0 {
		// Advertising data command takes exactly 31 bytes data, including manufacture data.
		// The length indicating the significant part of the data.
		data := [31]byte{}
		n := copy(data[:31], append(a.advertisingPacket, a.manufacturerData...))
		if err := a.cmd.SendAndCheckResp(
			cmd.LESetAdvertisingData{
				AdvertisingDataLength: uint8(n),
				AdvertisingData:       data,
			}, []byte{0x00}); err != nil {
			return err
		}
	}

	return nil
}

type Option func(*advertiser) Option

// Option sets the options specified.
func (a *advertiser) Option(opts ...Option) (prev Option) {
	for _, opt := range opts {
		prev = opt(a)
	}
	a.AdvertiseService()
	return prev
}

// AdvertisingPacket is an optional custom advertising packet.
// If nil, the advertising data will constructed to advertise
// as many services as possible. The AdvertisingPacket must be no
// longer than MaxAdvertisingPacketLength.
// If ManufacturerData is also set, their total length must be no
// longer than MaxAdvertisingPacketLength.
func AdvertisingPacket(b []byte) Option {
	return func(a *advertiser) Option {
		prev := a.advertisingPacket
		a.advertisingPacket = b
		return AdvertisingPacket(prev)
	}
}

// ScanResponsePacket is an optional custom scan response packet.
// If nil, the scan response packet will set to return the server
// name, truncated if necessary. The ScanResponsePacket must be no
// longer than MaxAdvertisingPacketLength.
func ScanResponsePacket(b []byte) Option {
	return func(a *advertiser) Option {
		prev := a.scanResponsePacket
		a.scanResponsePacket = b
		return ScanResponsePacket(prev)
	}
}

// ManufacturerData is an optional custom data.
// If set, it will be appended in the advertising data.
// The length of AdvertisingPacket ManufactureData must be no longer
// than MaxAdvertisingPacketLength .
func ManufacturerData(b []byte) Option {
	return func(a *advertiser) Option {
		prev := a.manufacturerData
		a.manufacturerData = b
		return ManufacturerData(prev)
	}
}

// AdvertisingIntervalMin is an optional parameter.
// If set, it overrides the default minimum advertising interval for
// undirected and low duty cycle directed advertising.
func AdvertisingIntervalMin(n uint16) Option {
	return func(a *advertiser) Option {
		prev := a.advertisingIntervalMin
		a.advertisingIntervalMin = n
		return AdvertisingIntervalMin(prev)
	}
}

// AdvertisingIntervalMax is an optional parameter.
// If set, it overrides the default maximum advertising interval for
// undirected and low duty cycle directed advertising.
func AdvertisingIntervalMax(n uint16) Option {
	return func(a *advertiser) Option {
		prev := a.advertisingIntervalMax
		a.advertisingIntervalMax = n
		return AdvertisingIntervalMax(prev)
	}
}

// AdvertisingChannelMap is an optional parameter.
// If set, it overrides the default advertising channel map.
func AdvertisingChannelMap(n uint8) Option {
	return func(a *advertiser) Option {
		prev := a.advertisingChannelMap
		a.advertisingChannelMap = n
		return AdvertisingChannelMap(prev)
	}
}
