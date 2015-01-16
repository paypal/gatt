package linux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type eventHandler interface {
	handleEvent([]byte) error
}

type handlerFunc func(b []byte) error

func (f handlerFunc) handleEvent(b []byte) error {
	return f(b)
}

type event struct {
	evtHandlers map[eventCode]eventHandler
}

func newEvent() *event {
	return &event{
		evtHandlers: map[eventCode]eventHandler{},
	}
}

func (e *event) handleEvent(c eventCode, h eventHandler) {
	e.evtHandlers[c] = h
}

func (e *event) dispatch(b []byte) error {
	h := &eventHeader{}
	if err := h.unmarshal(b); err != nil {
		return err
	}
	b = b[2:] // Skip Event Header (uint8 + uint8)
	if f, found := e.evtHandlers[h.code]; found {
		e.trace("> HCI Event: %s (0x%02X) plen %d: [ % X ])\n", h.code, uint8(h.code), h.plen, b)
		return f.handleEvent(b)
	}
	e.trace("> HCI Event: no handler for %s (0x%02X)\n", h.code, uint8(h.code))
	return nil
}

func (e *event) trace(fmt string, v ...interface{}) {}

type eventCode uint8

const (
	inquiryComplete                              eventCode = 0x01
	inquiryResult                                          = 0x02
	connectionComplete                                     = 0x03
	connectionRequest                                      = 0x04
	disconnectionComplete                                  = 0x05
	authenticationComplete                                 = 0x06
	remoteNameReqComplete                                  = 0x07
	encryptionChange                                       = 0x08
	changeConnectionLinkKeyComplete                        = 0x09
	masterLinkKeyComplete                                  = 0x0A
	readRemoteSupportedFeaturesComplete                    = 0x0B
	readRemoteVersionInformationComplete                   = 0x0C
	qosSetupComplete                                       = 0x0D
	commandComplete                                        = 0x0E
	commandStatus                                          = 0x0F
	hardwareError                                          = 0x10
	flushOccurred                                          = 0x11
	roleChange                                             = 0x12
	numberOfCompletedPkts                                  = 0x13
	modeChange                                             = 0x14
	returnLinkKeys                                         = 0x15
	pinCodeRequest                                         = 0x16
	linkKeyRequest                                         = 0x17
	linkKeyNotification                                    = 0x18
	loopbackCommand                                        = 0x19
	dataBufferOverflow                                     = 0x1A
	maxSlotsChange                                         = 0x1B
	readClockOffsetComplete                                = 0x1C
	connectionPtypeChanged                                 = 0x1D
	qosViolation                                           = 0x1E
	pageScanRepetitionModeChange                           = 0x20
	flowSpecificationComplete                              = 0x21
	inquiryResultWithRssi                                  = 0x22
	readRemoteExtendedFeaturesComplete                     = 0x23
	syncConnectionComplete                                 = 0x2C
	syncConnectionChanged                                  = 0x2D
	sniffSubrating                                         = 0x2E
	extendedInquiryResult                                  = 0x2F
	encryptionKeyRefreshComplete                           = 0x30
	ioCapabilityRequest                                    = 0x31
	ioCapabilityResponse                                   = 0x32
	userConfirmationRequest                                = 0x33
	userPasskeyRequest                                     = 0x34
	remoteOOBDataRequest                                   = 0x35
	simplePairingComplete                                  = 0x36
	linkSupervisionTimeoutChanged                          = 0x38
	enhancedFlushComplete                                  = 0x39
	userPasskeyNotify                                      = 0x3B
	keypressNotify                                         = 0x3C
	remoteHostFeaturesNotify                               = 0x3D
	leMeta                                                 = 0x3E
	physicalLinkComplete                                   = 0x40
	channelSelected                                        = 0x41
	disconnectionPhysicalLinkComplete                      = 0x42
	physicalLinkLossEarlyWarning                           = 0x43
	physicalLinkRecovery                                   = 0x44
	logicalLinkComplete                                    = 0x45
	disconnectionLogicalLinkComplete                       = 0x46
	flowSpecModifyComplete                                 = 0x47
	numberOfCompletedBlocks                                = 0x48
	ampStartTest                                           = 0x49
	ampTestEnd                                             = 0x4A
	ampReceiverReport                                      = 0x4b
	ampStatusChange                                        = 0x4D
	triggeredClockCapture                                  = 0x4e
	synchronizationTrainComplete                           = 0x4F
	synchronizationTrainReceived                           = 0x50
	connectionlessSlaveBroadcastReceive                    = 0x51
	connectionlessSlaveBroadcastTimeout                    = 0x52
	truncatedPageComplete                                  = 0x53
	slavePageResponseTimeout                               = 0x54
	connectionlessSlaveBroadcastChannelMapChange           = 0x55
	inquiryResponseNotification                            = 0x56
	authenticatedPayloadTimeoutExpired                     = 0x57
)

var eventName = map[eventCode]string{
	inquiryComplete:                      "Inquiry Complete",
	inquiryResult:                        "Inquiry Result",
	connectionComplete:                   "Connection Complete",
	connectionRequest:                    "Connection Request",
	disconnectionComplete:                "Disconnection Complete",
	authenticationComplete:               "Authentication",
	remoteNameReqComplete:                "Remote Name Request Complete",
	encryptionChange:                     "Encryption Change",
	changeConnectionLinkKeyComplete:      "Change Conection Link Key Complete",
	masterLinkKeyComplete:                "Master Link Keye Complete",
	readRemoteSupportedFeaturesComplete:  "Read Remote Supported Features Complete",
	readRemoteVersionInformationComplete: "Read Remote Version Information Complete",
	qosSetupComplete:                     "QoSSetupComplete",
	commandComplete:                      "Command Complete",
	commandStatus:                        "Command status",
	hardwareError:                        "Hardware Error",
	flushOccurred:                        "Flush Occured",
	roleChange:                           "Role Change",
	numberOfCompletedPkts:                "Number Of Completed Packets",
	modeChange:                           "Mode Change",
	returnLinkKeys:                       "Return Link Keys",
	pinCodeRequest:                       "PIN Code Request",
	linkKeyRequest:                       "Link Key Request",
	linkKeyNotification:                  "Link Key Notification",
	loopbackCommand:                      "Loopback Command",
	dataBufferOverflow:                   "Data Buffer Overflow",
	maxSlotsChange:                       "Max Slots Change",
	readClockOffsetComplete:              "Read Clock Offset Complete",
	connectionPtypeChanged:               "Connection Packet Type Changed",
	qosViolation:                         "QoS Violation",
	pageScanRepetitionModeChange:         "Page Scan Repetition Mode Change",
	flowSpecificationComplete:            "Flow Specification",
	inquiryResultWithRssi:                "Inquery Result with RSSI",
	readRemoteExtendedFeaturesComplete:   "Read Remote Extended Features Complete",
	syncConnectionComplete:               "Synchronous Connection Complete",
	syncConnectionChanged:                "Synchronous Connection Changed",
	sniffSubrating:                       "Sniff Subrating",
	extendedInquiryResult:                "Extended Inquiry Result",
	encryptionKeyRefreshComplete:         "Encryption Key Refresh Complete",
	ioCapabilityRequest:                  "IO Capability Request",
	ioCapabilityResponse:                 "IO Capability Changed",
	userConfirmationRequest:              "User Confirmation Request",
	userPasskeyRequest:                   "User Passkey Request",
	remoteOOBDataRequest:                 "Remote OOB Data",
	simplePairingComplete:                "Simple Pairing Complete",
	linkSupervisionTimeoutChanged:        "Link Supervision Timeout Changed",
	enhancedFlushComplete:                "Enhanced Flush Complete",
	userPasskeyNotify:                    "User Passkey Notification",
	keypressNotify:                       "Keypass Notification",
	remoteHostFeaturesNotify:             "Remote Host Supported Features Notification",
	leMeta:                                       "LE Meta",
	physicalLinkComplete:                         "Physical Link Complete",
	channelSelected:                              "Channel Selected",
	disconnectionPhysicalLinkComplete:            "Disconnection Physical Link Complete",
	physicalLinkLossEarlyWarning:                 "Physical Link Loss Early Warning",
	physicalLinkRecovery:                         "Physical Link Recovery",
	logicalLinkComplete:                          "Logical Link Complete",
	disconnectionLogicalLinkComplete:             "Disconnection Logical Link Complete",
	flowSpecModifyComplete:                       "Flow Spec Modify Complete",
	numberOfCompletedBlocks:                      "Number Of Completed Data Blocks",
	ampStartTest:                                 "AMP Start Test",
	ampTestEnd:                                   "AMP Test End",
	ampReceiverReport:                            "AMP Receiver Report",
	ampStatusChange:                              "AMP status Change",
	triggeredClockCapture:                        "Triggered Clock Capture",
	synchronizationTrainComplete:                 "Synchronization Train Complete",
	synchronizationTrainReceived:                 "Synchronization Train Received",
	connectionlessSlaveBroadcastReceive:          "Connectionless Slave Broadcast Receive",
	connectionlessSlaveBroadcastTimeout:          "Connectionless Slave Broadcast Timeout",
	truncatedPageComplete:                        "Truncated Page Complete",
	slavePageResponseTimeout:                     "Slave Page Response Timeout",
	connectionlessSlaveBroadcastChannelMapChange: "Connectionless Slave Broadcast Channel Map Change",
	inquiryResponseNotification:                  "Inquiry Response Notification",
	authenticatedPayloadTimeoutExpired:           "Authenticated Payload Timeout Expired",
}

func (e eventCode) String() string { return eventName[e] }

type leEventCode eventCode

const (
	leConnectionComplete               leEventCode = 0x01
	leAdvertisingReport                            = 0x02
	leConnectionUpdateComplete                     = 0x03
	leReadRemoteUsedFeaturesComplete               = 0x04
	leLTKRequest                                   = 0x05
	leRemoteConnectionParameterRequest             = 0x06
)

var leEventName = map[leEventCode]string{
	leConnectionComplete:               "LE Connection Complete",
	leAdvertisingReport:                "LE Advertising Report",
	leConnectionUpdateComplete:         "LE Connection Update Complete",
	leReadRemoteUsedFeaturesComplete:   "LE Read Remote Used Features Complete",
	leLTKRequest:                       "LE LTK Request",
	leRemoteConnectionParameterRequest: "LE Remote Connection Parameter Request",
}

func (e leEventCode) String() string { return leEventName[e] }

type eventHeader struct {
	code eventCode
	plen uint8
}

func (h *eventHeader) unmarshal(b []byte) error {
	if len(b) < 2 {
		return errors.New("malformed header")
	}
	h.code = eventCode(b[0])
	h.plen = b[1]
	if uint8(len(b)) != 2+h.plen {
		return errors.New("wrong length")
	}
	return nil
}

func (h *eventHeader) String() string {
	return fmt.Sprintf("> HCI Event: %s (0x%02X) plen: %02X", h.code, uint8(h.code), h.plen)
}

// Event Parameters

type inquiryCompleteEP struct {
	status uint8
}

type InquiryResultEP struct {
	numResponses           uint8
	bdaddr                 [][6]byte
	pageScanRepetitionMode []uint8
	reserved1              []byte
	reserved2              []byte
	classOfDevice          [][3]byte
	clockOffset            []uint16
}

type connectionCompleteEP struct {
	status            uint8
	connectionHandle  uint16
	bdaddr            [6]byte
	linkType          uint8
	encryptionEnabled uint8
}

type connectionRequestEP struct {
	bdaddr        [6]byte
	classofDevice [3]byte
	linkType      uint8
}

type disconnectionCompleteEP struct {
	status           uint8
	connectionHandle uint16
	reason           uint8
}

func (ep *disconnectionCompleteEP) unmarshal(b []byte) error {
	buf := bytes.NewBuffer(b)
	binary.Read(buf, binary.LittleEndian, &ep.status)
	binary.Read(buf, binary.LittleEndian, &ep.connectionHandle)
	return binary.Read(buf, binary.LittleEndian, &ep.reason)
}

type commandCompleteEP struct {
	numHCICommandPackets uint8
	commandOPCode        uint16
	returnParameters     []byte
}

func (ep *commandCompleteEP) unmarshal(b []byte) error {
	buf := bytes.NewBuffer(b)
	if err := binary.Read(buf, binary.LittleEndian, &ep.numHCICommandPackets); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.LittleEndian, &ep.commandOPCode); err != nil {
		return err
	}
	ep.returnParameters = buf.Bytes()
	return nil
}

type commandStatusEP struct {
	status               uint8
	numHCICommandPackets uint8
	commandOpcode        uint16
}

func (ep *commandStatusEP) unmarshal(b []byte) error {
	buf := bytes.NewBuffer(b)
	binary.Read(buf, binary.LittleEndian, &ep.status)
	binary.Read(buf, binary.LittleEndian, &ep.numHCICommandPackets)
	return binary.Read(buf, binary.LittleEndian, &ep.commandOpcode)
}

type numOfCompletedPkt struct {
	connectionHandle   uint16
	numOfCompletedPkts uint16
}

type numberOfCompletedPktsEP struct {
	numberOfHandles uint8
	packets         []numOfCompletedPkt
}

func (ep *numberOfCompletedPktsEP) unmarshal(b []byte) error {
	ep.numberOfHandles = b[0]
	n := int(ep.numberOfHandles)
	buf := bytes.NewBuffer(b[1:])
	ep.packets = make([]numOfCompletedPkt, n)
	for i := 0; i < n; i++ {
		binary.Read(buf, binary.LittleEndian, &ep.packets[i].connectionHandle)
		binary.Read(buf, binary.LittleEndian, &ep.packets[i].numOfCompletedPkts)

		ep.packets[i].connectionHandle &= 0xfff
	}
	return nil
}

// LE Meta Subevents
type leConnectionCompleteEP struct {
	subeventCode        uint8
	status              uint8
	connectionHandle    uint16
	role                uint8
	peerAddressType     uint8
	peerAddress         [6]byte
	connInterval        uint16
	connLatency         uint16
	supervisionTimeout  uint16
	masterClockAccuracy uint8
}

func (ep *leConnectionCompleteEP) unmarshal(b []byte) error {
	ep.subeventCode = o.Uint8(b[0:])
	ep.status = o.Uint8(b[1:])
	ep.connectionHandle = o.Uint16(b[2:])
	ep.role = o.Uint8(b[4:])
	ep.peerAddressType = o.Uint8(b[5:])
	ep.peerAddress = o.MAC(b[6:])
	ep.connInterval = o.Uint16(b[12:])
	ep.connLatency = o.Uint16(b[14:])
	ep.supervisionTimeout = o.Uint16(b[16:])
	ep.masterClockAccuracy = o.Uint8(b[17:])
	return nil
}

type leAdvertisingReportEP struct {
	subeventCode uint8
	numReports   uint8
	eventType    []uint8
	addressType  []uint8
	address      [][6]byte
	length       []uint8
	data         [][]byte
	rssi         []int8
}

func (ep *leAdvertisingReportEP) unmarshal(b []byte) error {
	ep.subeventCode = o.Uint8(b)
	b = b[1:]
	ep.numReports = o.Uint8(b)
	b = b[1:]
	n := int(ep.numReports)
	ep.eventType = make([]uint8, n)
	ep.addressType = make([]uint8, n)
	ep.address = make([][6]byte, n)
	ep.length = make([]uint8, n)
	ep.data = make([][]byte, n)
	ep.rssi = make([]int8, n)

	for i := 0; i < n; i++ {
		ep.eventType[i] = o.Uint8(b)
		b = b[1:]
	}
	for i := 0; i < n; i++ {
		ep.addressType[i] = o.Uint8(b)
		b = b[1:]
	}
	for i := 0; i < n; i++ {
		ep.address[i] = o.MAC(b)
		b = b[6:]
	}
	for i := 0; i < n; i++ {
		ep.length[i] = o.Uint8(b)
		b = b[1:]
	}
	for i := 0; i < n; i++ {
		ep.data[i] = make([]byte, ep.length[i])
		copy(ep.data[i], b)
		b = b[ep.length[i]:]
	}
	for i := 0; i < n; i++ {
		ep.rssi[i] = o.Int8(b)
		b = b[1:]
	}
	return nil
}

type leConnectionUpdateCompleteEP struct {
	subeventCode       uint8
	status             uint8
	connectionHandle   uint16
	connInterval       uint16
	connLatency        uint16
	supervisionTimeout uint16
}

func (ep *leConnectionUpdateCompleteEP) unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type leReadRemoteUsedFeaturesCompleteEP struct {
	subeventCode     uint8
	status           uint8
	connectionHandle uint16
	leFeatures       uint64
}

func (ep *leReadRemoteUsedFeaturesCompleteEP) unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type leLTKRequestEP struct {
	subeventCode          uint8
	connectionHandle      uint16
	randomNumber          uint64
	encryptionDiversifier uint16
}

func (ep *leLTKRequestEP) unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type leRemoteConnectionParameterRequestEP struct {
	subeventCode     uint8
	connectionHandle uint16
	intervalMin      uint16
	intervalMax      uint16
	latency          uint16
	timeout          uint16
}

func (ep *leRemoteConnectionParameterRequestEP) unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}
