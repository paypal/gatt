package linux

type packetType uint8

// HCI Packet types
const (
	typCommandPkt packetType = 0x01
	typACLDataPkt            = 0x02
	typSCODataPkt            = 0x03
	typEventPkt              = 0x04
	typVendorPkt             = 0xFF
)

// Event Type
const (
	advInd        = 0x00 // Connectable undirected advertising (ADV_IND).
	advDirectInd  = 0x01 // Connectable directed advertising (ADV_DIRECT_IND)
	advScanInd    = 0x02 // Scannable undirected advertising (ADV_SCAN_IND)
	advNonconnInd = 0x03 // Non connectable undirected advertising (ADV_NONCONN_IND)
	scanRsp       = 0x04 // Scan Response (SCAN_RSP)
)
