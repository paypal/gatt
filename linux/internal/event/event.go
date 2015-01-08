package event

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type EventHandler interface {
	HandleEvent([]byte) error
}

type HandlerFunc func(b []byte) error

func (f HandlerFunc) HandleEvent(b []byte) error {
	return f(b)
}

type Event struct {
	evtHandlers    map[EventCode]EventHandler
	defaultHandler EventHandler
}

func NewEvent() *Event {
	return &Event{
		evtHandlers:    map[EventCode]EventHandler{},
		defaultHandler: nil,
	}
}

func (e *Event) HandleEvent(c EventCode, h EventHandler) {
	e.evtHandlers[c] = h
}

func (e *Event) HandleEventDefault(h EventHandler) {
	e.defaultHandler = h
}

func (e *Event) Dispatch(b []byte) error {
	h := &EventHeader{}
	if err := h.Unmarshal(b); err != nil {
		return err
	}
	b = b[2:] // Skip Event Header (uint8 + uint8)
	if f, found := e.evtHandlers[h.Code]; found {
		e.trace("> HCI Event: %s (0x%02X) plen %d: [ % X ])\n", h.Code, uint8(h.Code), h.Plen, b)
		return f.HandleEvent(b)
	}
	if e.defaultHandler != nil {
		e.trace("> HCI Event: default handler for %s (0x%02X): [ % X ])\n", h.Code, uint8(h.Code), b)
		return e.defaultHandler.HandleEvent(b)
	}
	e.trace("> HCI Event: no handler for %s (0x%02X)\n", h.Code, uint8(h.Code))
	return nil
}

func (e *Event) trace(fmt string, v ...interface{}) {}

type EventCode uint8

const (
	InquiryComplete                              EventCode = 0x01
	InquiryResult                                          = 0x02
	ConnectionComplete                                     = 0x03
	ConnectionRequest                                      = 0x04
	DisconnectionComplete                                  = 0x05
	AuthenticationComplete                                 = 0x06
	RemoteNameReqComplete                                  = 0x07
	EncryptionChange                                       = 0x08
	ChangeConnectionLinkKeyComplete                        = 0x09
	MasterLinkKeyComplete                                  = 0x0A
	ReadRemoteSupportedFeaturesComplete                    = 0x0B
	ReadRemoteVersionInformationComplete                   = 0x0C
	QOSSetupComplete                                       = 0x0D
	CommandComplete                                        = 0x0E
	CommandStatus                                          = 0x0F
	HardwareError                                          = 0x10
	FlushOccurred                                          = 0x11
	RoleChange                                             = 0x12
	NumberOfCompletedPkts                                  = 0x13
	ModeChange                                             = 0x14
	ReturnLinkKeys                                         = 0x15
	PINCodeRequest                                         = 0x16
	LinkKeyRequest                                         = 0x17
	LinkKeyNotification                                    = 0x18
	LoopbackCommand                                        = 0x19
	DataBufferOverflow                                     = 0x1A
	MaxSlotsChange                                         = 0x1B
	ReadClockOffsetComplete                                = 0x1C
	ConnectionPtypeChanged                                 = 0x1D
	QOSViolation                                           = 0x1E
	PageScanRepetitionModeChange                           = 0x20
	FlowSpecificationComplete                              = 0x21
	InquiryResultWithRssi                                  = 0x22
	ReadRemoteExtendedFeaturesComplete                     = 0x23
	SyncConnectionComplete                                 = 0x2C
	SyncConnectionChanged                                  = 0x2D
	SniffSubrating                                         = 0x2E
	ExtendedInquiryResult                                  = 0x2F
	EncryptionKeyRefreshComplete                           = 0x30
	IOCapabilityRequest                                    = 0x31
	IOCapabilityResponse                                   = 0x32
	UserConfirmationRequest                                = 0x33
	UserPasskeyRequest                                     = 0x34
	RemoteOOBDataRequest                                   = 0x35
	SimplePairingComplete                                  = 0x36
	LinkSupervisionTimeoutChanged                          = 0x38
	EnhancedFlushComplete                                  = 0x39
	UserPasskeyNotify                                      = 0x3B
	KeypressNotify                                         = 0x3C
	RemoteHostFeaturesNotify                               = 0x3D
	LEMeta                                                 = 0x3E
	PhysicalLinkComplete                                   = 0x40
	ChannelSelected                                        = 0x41
	DisconnectionPhysicalLinkComplete                      = 0x42
	PhysicalLinkLossEarlyWarning                           = 0x43
	PhysicalLinkRecovery                                   = 0x44
	LogicalLinkComplete                                    = 0x45
	DisconnectionLogicalLinkComplete                       = 0x46
	FlowSpecModifyComplete                                 = 0x47
	NumberOfCompletedBlocks                                = 0x48
	AMPStartTest                                           = 0x49
	AMPTestEnd                                             = 0x4A
	AMPReceiverReport                                      = 0x4b
	AMPStatusChange                                        = 0x4D
	TriggeredClockCapture                                  = 0x4e
	SynchronizationTrainComplete                           = 0x4F
	SynchronizationTrainReceived                           = 0x50
	ConnectionlessSlaveBroadcastReceive                    = 0x51
	ConnectionlessSlaveBroadcastTimeout                    = 0x52
	TruncatedPageComplete                                  = 0x53
	SlavePageResponseTimeout                               = 0x54
	ConnectionlessSlaveBroadcastChannelMapChange           = 0x55
	InquiryResponseNotification                            = 0x56
	AuthenticatedPayloadTimeoutExpired                     = 0x57
)

var eventName = map[EventCode]string{
	InquiryComplete:                      "Inquiry Complete",
	InquiryResult:                        "Inquiry Result",
	ConnectionComplete:                   "Connection Complete",
	ConnectionRequest:                    "Connection Request",
	DisconnectionComplete:                "Disconnection Complete",
	AuthenticationComplete:               "Authentication",
	RemoteNameReqComplete:                "Remote Name Request Complete",
	EncryptionChange:                     "Encryption Change",
	ChangeConnectionLinkKeyComplete:      "Change Conection Link Key Complete",
	MasterLinkKeyComplete:                "Master Link Keye Complete",
	ReadRemoteSupportedFeaturesComplete:  "Read Remote Supported Features Complete",
	ReadRemoteVersionInformationComplete: "Read Remote Version Information Complete",
	QOSSetupComplete:                     "QoSSetupComplete",
	CommandComplete:                      "Command Complete",
	CommandStatus:                        "Command Status",
	HardwareError:                        "Hardware Error",
	FlushOccurred:                        "Flush Occured",
	RoleChange:                           "Role Change",
	NumberOfCompletedPkts:                "Number Of Completed Packets",
	ModeChange:                           "Mode Change",
	ReturnLinkKeys:                       "Return Link Keys",
	PINCodeRequest:                       "PIN Code Request",
	LinkKeyRequest:                       "Link Key Request",
	LinkKeyNotification:                  "Link Key Notification",
	LoopbackCommand:                      "Loopback Command",
	DataBufferOverflow:                   "Data Buffer Overflow",
	MaxSlotsChange:                       "Max Slots Change",
	ReadClockOffsetComplete:              "Read Clock Offset Complete",
	ConnectionPtypeChanged:               "Connection Packet Type Changed",
	QOSViolation:                         "QoS Violation",
	PageScanRepetitionModeChange:         "Page Scan Repetition Mode Change",
	FlowSpecificationComplete:            "Flow Specification",
	InquiryResultWithRssi:                "Inquery Result with RSSI",
	ReadRemoteExtendedFeaturesComplete:   "Read Remote Extended Features Complete",
	SyncConnectionComplete:               "Synchronous Connection Complete",
	SyncConnectionChanged:                "Synchronous Connection Changed",
	SniffSubrating:                       "Sniff Subrating",
	ExtendedInquiryResult:                "Extended Inquiry Result",
	EncryptionKeyRefreshComplete:         "Encryption Key Refresh Complete",
	IOCapabilityRequest:                  "IO Capability Request",
	IOCapabilityResponse:                 "IO Capability Changed",
	UserConfirmationRequest:              "User Confirmation Request",
	UserPasskeyRequest:                   "User Passkey Request",
	RemoteOOBDataRequest:                 "Remote OOB Data",
	SimplePairingComplete:                "Simple Pairing Complete",
	LinkSupervisionTimeoutChanged:        "Link Supervision Timeout Changed",
	EnhancedFlushComplete:                "Enhanced Flush Complete",
	UserPasskeyNotify:                    "User Passkey Notification",
	KeypressNotify:                       "Keypass Notification",
	RemoteHostFeaturesNotify:             "Remote Host Supported Features Notification",
	LEMeta:                                       "LE Meta",
	PhysicalLinkComplete:                         "Physical Link Complete",
	ChannelSelected:                              "Channel Selected",
	DisconnectionPhysicalLinkComplete:            "Disconnection Physical Link Complete",
	PhysicalLinkLossEarlyWarning:                 "Physical Link Loss Early Warning",
	PhysicalLinkRecovery:                         "Physical Link Recovery",
	LogicalLinkComplete:                          "Logical Link Complete",
	DisconnectionLogicalLinkComplete:             "Disconnection Logical Link Complete",
	FlowSpecModifyComplete:                       "Flow Spec Modify Complete",
	NumberOfCompletedBlocks:                      "Number Of Completed Data Blocks",
	AMPStartTest:                                 "AMP Start Test",
	AMPTestEnd:                                   "AMP Test End",
	AMPReceiverReport:                            "AMP Receiver Report",
	AMPStatusChange:                              "AMP Status Change",
	TriggeredClockCapture:                        "Triggered Clock Capture",
	SynchronizationTrainComplete:                 "Synchronization Train Complete",
	SynchronizationTrainReceived:                 "Synchronization Train Received",
	ConnectionlessSlaveBroadcastReceive:          "Connectionless Slave Broadcast Receive",
	ConnectionlessSlaveBroadcastTimeout:          "Connectionless Slave Broadcast Timeout",
	TruncatedPageComplete:                        "Truncated Page Complete",
	SlavePageResponseTimeout:                     "Slave Page Response Timeout",
	ConnectionlessSlaveBroadcastChannelMapChange: "Connectionless Slave Broadcast Channel Map Change",
	InquiryResponseNotification:                  "Inquiry Response Notification",
	AuthenticatedPayloadTimeoutExpired:           "Authenticated Payload Timeout Expired",
}

func (e EventCode) String() string { return eventName[e] }

type LEEventCode EventCode

const (
	LEConnectionComplete               LEEventCode = 0x01
	LEAdvertisingReport                            = 0x02
	LEConnectionUpdateComplete                     = 0x03
	LEReadRemoteUsedFeaturesComplete               = 0x04
	LELTKRequest                                   = 0x05
	LERemoteConnectionParameterRequest             = 0x06
)

var leEventName = map[LEEventCode]string{
	LEConnectionComplete:               "LE Connection Complete",
	LEAdvertisingReport:                "LE Advertising Report",
	LEConnectionUpdateComplete:         "LE Connection Update Complete",
	LEReadRemoteUsedFeaturesComplete:   "LE Read Remote Used Features Complete",
	LELTKRequest:                       "LE LTK Request",
	LERemoteConnectionParameterRequest: "LE Remote Connection Parameter Request",
}

func (e LEEventCode) String() string { return leEventName[e] }

type EventHeader struct {
	Code EventCode
	Plen uint8
}

func (h *EventHeader) Unmarshal(b []byte) error {
	if len(b) < 2 {
		return errors.New("malformed header")
	}
	h.Code = EventCode(b[0])
	h.Plen = b[1]
	if uint8(len(b)) != 2+h.Plen {
		return errors.New("wrong length")
	}
	return nil
}

func (h *EventHeader) String() string {
	return fmt.Sprintf("> HCI Event: %s (0x%02X) plen: %02X", h.Code, uint8(h.Code), h.Plen)
}

// Event Parameters

type InquiryCompleteEP struct {
	Status uint8
}

type InquiryResultEP struct {
	NumResponses           uint8
	BDADDR                 [][6]byte
	PageScanRepetitionMode []uint8
	Reserved1              []byte
	Reserved2              []byte
	ClassOfDevice          [][3]byte
	ClockOffset            []uint16
}

type ConnectionCompleteEP struct {
	Status            uint8
	ConnectionHandle  uint16
	BDADDR            [6]byte
	LinkType          uint8
	EncryptionEnabled uint8
}

type ConnectionRequestEP struct {
	BD_ADDR       [6]byte
	ClassofDevice [3]byte
	LinkType      uint8
}

type DisconnectionCompleteEP struct {
	Status           uint8
	ConnectionHandle uint16
	Reason           uint8
}

func (ep *DisconnectionCompleteEP) Unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type CommandCompleteEP struct {
	NumHCICommandPackets uint8
	CommandOPCode        uint16
	ReturnParameters     []byte
}

func (ep *CommandCompleteEP) Unmarshal(b []byte) error {
	buf := bytes.NewBuffer(b)
	if err := binary.Read(buf, binary.LittleEndian, &ep.NumHCICommandPackets); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.LittleEndian, &ep.CommandOPCode); err != nil {
		return err
	}
	ep.ReturnParameters = buf.Bytes()
	return nil
}

type CommandStatusEP struct {
	Status               uint8
	NumHCICommandPackets uint8
	CommandOpcode        uint16
}

func (ep *CommandStatusEP) Unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type NumOfCompletedPkt struct {
	ConnectionHandle   uint16
	NumOfCompletedPkts uint16
}

type NumberOfCompletedPktsEP struct {
	NumberOfHandles uint8
	Packets         []NumOfCompletedPkt
}

func (ep *NumberOfCompletedPktsEP) Unmarshal(b []byte) error {
	ep.NumberOfHandles = b[0]
	n := int(ep.NumberOfHandles)
	buf := bytes.NewBuffer(b[1:])
	ep.Packets = make([]NumOfCompletedPkt, n)
	for i := 0; i < n; i++ {
		if err := binary.Read(buf, binary.LittleEndian, &ep.Packets[i]); err != nil {
			return err
		}
		ep.Packets[i].ConnectionHandle &= 0xfff
	}
	return nil
}

// LE Meta Subevents
type LEConnectionCompleteEP struct {
	SubeventCode        uint8
	Status              uint8
	ConnectionHandle    uint16
	Role                uint8
	PeerAddressType     uint8
	PeerAddress         [6]byte
	ConnInterval        uint16
	ConnLatency         uint16
	SupervisionTimeout  uint16
	MasterClockAccuracy uint8
}

func (ep *LEConnectionCompleteEP) Unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type LEAdvertisingReportEP struct {
	SubeventCode uint8
	NumReports   uint8
	EventType    []uint8
	AddressType  []uint8
	Address      [][6]byte
	Length       []uint8
	Data         [][]byte
	RSSI         []int8
}

func (ep *LEAdvertisingReportEP) Unmarshal(b []byte) error {
	buf := bytes.NewBuffer(b)
	binary.Read(buf, binary.LittleEndian, &ep.SubeventCode)
	binary.Read(buf, binary.LittleEndian, &ep.NumReports)
	n := int(ep.NumReports)
	ep.EventType = make([]uint8, n)
	ep.AddressType = make([]uint8, n)
	ep.Address = make([][6]byte, n)
	ep.Length = make([]uint8, n)
	ep.Data = make([][]byte, n)
	ep.RSSI = make([]int8, n)
	for i := 0; i < n; i++ {
		binary.Read(buf, binary.LittleEndian, &ep.EventType[i])
	}
	for i := 0; i < n; i++ {
		binary.Read(buf, binary.LittleEndian, &ep.AddressType[i])
	}
	for i := 0; i < n; i++ {
		binary.Read(buf, binary.LittleEndian, &ep.Address[i])
	}
	for i := 0; i < n; i++ {
		binary.Read(buf, binary.LittleEndian, &ep.Length[i])
	}
	for i := 0; i < n; i++ {
		ep.Data[i] = make([]byte, ep.Length[i])
		binary.Read(buf, binary.LittleEndian, &ep.Data[i])
	}
	for i := 0; i < n; i++ {
		binary.Read(buf, binary.LittleEndian, &ep.RSSI[i])
	}
	return nil
}

type LEConnectionUpdateCompleteEP struct {
	SubeventCode       uint8
	Status             uint8
	ConnectionHandle   uint16
	ConnInterval       uint16
	ConnLatency        uint16
	SupervisionTimeout uint16
}

func (ep *LEConnectionUpdateCompleteEP) Unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type LEReadRemoteUsedFeaturesCompleteEP struct {
	SubeventCode     uint8
	Status           uint8
	ConnectionHandle uint16
	LEFeatures       uint64
}

func (ep *LEReadRemoteUsedFeaturesCompleteEP) Unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type LELTKRequestEP struct {
	SubeventCode          uint8
	ConnectionHandle      uint16
	RandomNumber          uint64
	EncryptionDiversifier uint16
}

func (ep *LELTKRequestEP) Unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}

type LERemoteConnectionParameterRequestEP struct {
	SubeventCode     uint8
	ConnectionHandle uint16
	IntervalMin      uint16
	IntervalMax      uint16
	Latency          uint16
	Timeout          uint16
}

func (ep *LERemoteConnectionParameterRequestEP) Unmarshal(b []byte) error {
	return binary.Read(bytes.NewBuffer(b), binary.LittleEndian, ep)
}
