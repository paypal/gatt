package hci

type PacketType uint8

// HCI Packet types
const (
	TypCommandPkt PacketType = 0X01
	TypACLDataPkt            = 0X02
	TypSCODataPkt            = 0X03
	TypEventPkt              = 0X04
	TypVendorPkt             = 0XFF
)
