package cmd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/paypal/gatt/hci/event"
)

const ptypeCommandPkt uint8 = 0x01

type CmdParam interface {
	Opcode() Opcode
	Marshal() []byte
}

func NewCmd(d io.Writer, l *log.Logger) *Cmd {
	c := &Cmd{
		dev:     d,
		logger:  l,
		sent:    []*cmdPkt{},
		compc:   make(chan *event.CommandCompleteEP),
		statusc: make(chan *event.CommandStatusEP),
	}
	go c.processCmdEvents()
	return c
}

type cmdPkt struct {
	op   Opcode
	cp   CmdParam
	done chan []byte
}

func (c cmdPkt) marshal() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.LittleEndian, ptypeCommandPkt); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, c.op); err != nil {
		return nil, err
	}
	b := c.cp.Marshal()
	if err := binary.Write(buf, binary.LittleEndian, uint8(len(b))); err != nil {
		return nil, err
	}
	if b == nil || len(b) == 0 {
		return buf.Bytes(), nil
	}
	if err := binary.Write(buf, binary.LittleEndian, b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type Cmd struct {
	dev     io.Writer
	logger  *log.Logger
	sent    []*cmdPkt
	compc   chan *event.CommandCompleteEP
	statusc chan *event.CommandStatusEP
}

func (c Cmd) trace(fmt string, v ...interface{}) {
	if c.logger == nil {
		return
	}
	c.logger.Printf(fmt, v...)
}

func (c *Cmd) HandleComplete(b []byte) error {
	ep := &event.CommandCompleteEP{}
	if err := ep.Unmarshal(b); err != nil {
		return err
	}
	c.compc <- ep
	return nil
}

func (c *Cmd) HandleStatus(b []byte) error {
	ep := &event.CommandStatusEP{}
	if err := ep.Unmarshal(b); err != nil {
		return err
	}
	c.statusc <- ep
	return nil
}

func (c *Cmd) Send(cp CmdParam) ([]byte, error) {
	op := cp.Opcode()
	p := &cmdPkt{op: op, cp: cp, done: make(chan []byte)}
	raw, err := p.marshal()
	if err != nil {
		return nil, err
	}

	c.trace("< HCI Command: %s (0x%02X|0x%04X) plen: %d [ % X ]\n", op, op.ogf(), uint16(op.ocf()), len(raw)-4, raw) // FIXME: plen
	c.sent = append(c.sent, p)
	if n, err := c.dev.Write(raw); err != nil {
		return nil, err
	} else if n != len(raw) {
		return nil, errors.New("Failed to send whole cmd pkt to HCI socket")
	}
	return <-p.done, nil
}

func (c *Cmd) SendAndCheckResp(cp CmdParam, exp []byte) error {
	rsp, err := c.Send(cp)
	if err != nil {
		return err
	}
	// Don't care about the response
	if len(exp) == 0 {
		return nil
	}
	// Check the if status is one of the expected value
	if !bytes.Contains(exp, rsp[0:1]) {
		return fmt.Errorf("HCI command: '%s' return 0x%02X, expect: [%X] ", cp.Opcode(), rsp[0], exp)
	}
	return nil
}

func (c *Cmd) processCmdEvents() {
	for {
		select {
		case status := <-c.statusc:
			found := false
			for i, p := range c.sent {
				if uint16(p.op) == status.CommandOpcode {
					found = true
					c.sent = append(c.sent[:i], c.sent[i+1:]...)
					close(p.done)
					break
				}
			}
			if !found {
				log.Printf("Can't find the cmdPkt for this CommandStatusEP: %v", status)
			}
		case comp := <-c.compc:
			found := false
			for i, p := range c.sent {
				if uint16(p.op) == comp.CommandOPCode {
					found = true
					c.sent = append(c.sent[:i], c.sent[i+1:]...)
					p.done <- comp.ReturnParameters
					break
				}
			}
			if !found {
				log.Printf("Can't find the cmdPkt for this CommandCompleteEP: %v", comp)
			}
		}
	}
}

const (
	LinkCtl     = 0x01
	LinkPolicy  = 0x02
	HostCtl     = 0x03
	InfoParam   = 0x04
	StatusParam = 0x05
	TestingCmd  = 0X3E
	LECtl       = 0x08
	VendorCmd   = 0X3F
)

type Opcode uint16

func (op Opcode) ogf() uint8     { return uint8((uint16(op) & 0xFC00) >> 10) }
func (op Opcode) ocf() uint16    { return uint16(op) & 0x03FF }
func (op Opcode) String() string { return opName[op] }

const (
	opInquiry                = Opcode(LinkCtl<<10 | 0x0001)
	opInquiryCancel          = Opcode(LinkCtl<<10 | 0x0002)
	opPeriodicInquiry        = Opcode(LinkCtl<<10 | 0x0003)
	opExitPeriodicInquiry    = Opcode(LinkCtl<<10 | 0x0004)
	opCreateConn             = Opcode(LinkCtl<<10 | 0x0005)
	opDisconnect             = Opcode(LinkCtl<<10 | 0x0006)
	opCreateConnCancel       = Opcode(LinkCtl<<10 | 0x0008)
	opAcceptConnReq          = Opcode(LinkCtl<<10 | 0x0009)
	opRejectConnReq          = Opcode(LinkCtl<<10 | 0x000A)
	opLinkKeyReply           = Opcode(LinkCtl<<10 | 0x000B)
	opLinkKeyNegReply        = Opcode(LinkCtl<<10 | 0x000C)
	opPinCodeReply           = Opcode(LinkCtl<<10 | 0x000D)
	opPinCodeNegReply        = Opcode(LinkCtl<<10 | 0x000E)
	opSetConnPtype           = Opcode(LinkCtl<<10 | 0x000F)
	opAuthRequested          = Opcode(LinkCtl<<10 | 0x0011)
	opSetConnEncrypt         = Opcode(LinkCtl<<10 | 0x0013)
	opChangeConnLinkKey      = Opcode(LinkCtl<<10 | 0x0015)
	opMasterLinkKey          = Opcode(LinkCtl<<10 | 0x0017)
	opRemoteNameReq          = Opcode(LinkCtl<<10 | 0x0019)
	opRemoteNameReqCancel    = Opcode(LinkCtl<<10 | 0x001A)
	opReadRemoteFeatures     = Opcode(LinkCtl<<10 | 0x001B)
	opReadRemoteExtFeatures  = Opcode(LinkCtl<<10 | 0x001C)
	opReadRemoteVersion      = Opcode(LinkCtl<<10 | 0x001D)
	opReadClockOffset        = Opcode(LinkCtl<<10 | 0x001F)
	opReadLMPHandle          = Opcode(LinkCtl<<10 | 0x0020)
	opSetupSyncConn          = Opcode(LinkCtl<<10 | 0x0028)
	opAcceptSyncConnReq      = Opcode(LinkCtl<<10 | 0x0029)
	opRejectSyncConnReq      = Opcode(LinkCtl<<10 | 0x002A)
	opIOCapabilityReply      = Opcode(LinkCtl<<10 | 0x002B)
	opUserConfirmReply       = Opcode(LinkCtl<<10 | 0x002C)
	opUserConfirmNegReply    = Opcode(LinkCtl<<10 | 0x002D)
	opUserPasskeyReply       = Opcode(LinkCtl<<10 | 0x002E)
	opUserPasskeyNegReply    = Opcode(LinkCtl<<10 | 0x002F)
	opRemoteOOBDataReply     = Opcode(LinkCtl<<10 | 0x0030)
	opRemoteOOBDataNegReply  = Opcode(LinkCtl<<10 | 0x0033)
	opIOCapabilityNegReply   = Opcode(LinkCtl<<10 | 0x0034)
	opCreatePhysicalLink     = Opcode(LinkCtl<<10 | 0x0035)
	opAcceptPhysicalLink     = Opcode(LinkCtl<<10 | 0x0036)
	opDisconnectPhysicalLink = Opcode(LinkCtl<<10 | 0x0037)
	opCreateLogicalLink      = Opcode(LinkCtl<<10 | 0x0038)
	opAcceptLogicalLink      = Opcode(LinkCtl<<10 | 0x0039)
	opDisconnectLogicalLink  = Opcode(LinkCtl<<10 | 0x003A)
	opLogicalLinkCancel      = Opcode(LinkCtl<<10 | 0x003B)
	opFlowSpecModify         = Opcode(LinkCtl<<10 | 0x003C)
)

const (
	opHoldMode               = Opcode(LinkPolicy<<10 | 0x0001)
	opSniffMode              = Opcode(LinkPolicy<<10 | 0x0003)
	opExitSniffMode          = Opcode(LinkPolicy<<10 | 0x0004)
	opParkMode               = Opcode(LinkPolicy<<10 | 0x0005)
	opExitParkMode           = Opcode(LinkPolicy<<10 | 0x0006)
	opQoSSetup               = Opcode(LinkPolicy<<10 | 0x0007)
	opRoleDiscovery          = Opcode(LinkPolicy<<10 | 0x0009)
	opSwitchRole             = Opcode(LinkPolicy<<10 | 0x000B)
	opReadLinkPolicy         = Opcode(LinkPolicy<<10 | 0x000C)
	opWriteLinkPolicy        = Opcode(LinkPolicy<<10 | 0x000D)
	opReadDefaultLinkPolicy  = Opcode(LinkPolicy<<10 | 0x000E)
	opWriteDefaultLinkPolicy = Opcode(LinkPolicy<<10 | 0x000F)
	opFlowSpecification      = Opcode(LinkPolicy<<10 | 0x0010)
	opSniffSubrating         = Opcode(LinkPolicy<<10 | 0x0011)
)
const (
	opSetEventMask                      = Opcode(HostCtl<<10 | 0x0001)
	opReset                             = Opcode(HostCtl<<10 | 0x0003)
	opSetEventFlt                       = Opcode(HostCtl<<10 | 0x0005)
	opFlush                             = Opcode(HostCtl<<10 | 0x0008)
	opReadPinType                       = Opcode(HostCtl<<10 | 0x0009)
	opWritePinType                      = Opcode(HostCtl<<10 | 0x000A)
	opCreateNewUnitKey                  = Opcode(HostCtl<<10 | 0x000B)
	opReadStoredLinkKey                 = Opcode(HostCtl<<10 | 0x000D)
	opWriteStoredLinkKey                = Opcode(HostCtl<<10 | 0x0011)
	opDeleteStoredLinkKey               = Opcode(HostCtl<<10 | 0x0012)
	opWriteLocalName                    = Opcode(HostCtl<<10 | 0x0013)
	opReadLocalName                     = Opcode(HostCtl<<10 | 0x0014)
	opReadConnAcceptTimeout             = Opcode(HostCtl<<10 | 0x0015)
	opWriteConnAcceptTimeout            = Opcode(HostCtl<<10 | 0x0016)
	opReadPageTimeout                   = Opcode(HostCtl<<10 | 0x0017)
	opWritePageTimeout                  = Opcode(HostCtl<<10 | 0x0018)
	opReadScanEnable                    = Opcode(HostCtl<<10 | 0x0019)
	opWriteScanEnable                   = Opcode(HostCtl<<10 | 0x001A)
	opReadPageActivity                  = Opcode(HostCtl<<10 | 0x001B)
	opWritePageActivity                 = Opcode(HostCtl<<10 | 0x001C)
	opReadInqActivity                   = Opcode(HostCtl<<10 | 0x001D)
	opWriteInqActivity                  = Opcode(HostCtl<<10 | 0x001E)
	opReadAuthEnable                    = Opcode(HostCtl<<10 | 0x001F)
	opWriteAuthEnable                   = Opcode(HostCtl<<10 | 0x0020)
	opReadEncryptMode                   = Opcode(HostCtl<<10 | 0x0021)
	opWriteEncryptMode                  = Opcode(HostCtl<<10 | 0x0022)
	opReadClassOfDev                    = Opcode(HostCtl<<10 | 0x0023)
	opWriteClassOfDevice                = Opcode(HostCtl<<10 | 0x0024)
	opReadVoiceSetting                  = Opcode(HostCtl<<10 | 0x0025)
	opWriteVoiceSetting                 = Opcode(HostCtl<<10 | 0x0026)
	opReadAutomaticFlushTimeout         = Opcode(HostCtl<<10 | 0x0027)
	opWriteAutomaticFlushTimeout        = Opcode(HostCtl<<10 | 0x0028)
	opReadNumBroadcastRetrans           = Opcode(HostCtl<<10 | 0x0029)
	opWriteNumBroadcastRetrans          = Opcode(HostCtl<<10 | 0x002A)
	opReadHoldModeActivity              = Opcode(HostCtl<<10 | 0x002B)
	opWriteHoldModeActivity             = Opcode(HostCtl<<10 | 0x002C)
	opReadTransmitPowerLevel            = Opcode(HostCtl<<10 | 0x002D)
	opReadSyncFlowEnable                = Opcode(HostCtl<<10 | 0x002E)
	opWriteSyncFlowEnable               = Opcode(HostCtl<<10 | 0x002F)
	opSetControllerToHostFC             = Opcode(HostCtl<<10 | 0x0031)
	opHostBufferSize                    = Opcode(HostCtl<<10 | 0x0033)
	opHostNumCompPkts                   = Opcode(HostCtl<<10 | 0x0035)
	opReadLinkSupervisionTimeout        = Opcode(HostCtl<<10 | 0x0036)
	opWriteLinkSupervisionTimeout       = Opcode(HostCtl<<10 | 0x0037)
	opReadNumSupportedIAC               = Opcode(HostCtl<<10 | 0x0038)
	opReadCurrentIACLAP                 = Opcode(HostCtl<<10 | 0x0039)
	opWriteCurrentIACLAP                = Opcode(HostCtl<<10 | 0x003A)
	opReadPageScanPeriodMode            = Opcode(HostCtl<<10 | 0x003B)
	opWritePageScanPeriodMode           = Opcode(HostCtl<<10 | 0x003C)
	opReadPageScanMode                  = Opcode(HostCtl<<10 | 0x003D)
	opWritePageScanMode                 = Opcode(HostCtl<<10 | 0x003E)
	opSetAFHClassification              = Opcode(HostCtl<<10 | 0x003F)
	opReadInquiryScanType               = Opcode(HostCtl<<10 | 0x0042)
	opWriteInquiryScanType              = Opcode(HostCtl<<10 | 0x0043)
	opReadInquiryMode                   = Opcode(HostCtl<<10 | 0x0044)
	opWriteInquiryMode                  = Opcode(HostCtl<<10 | 0x0045)
	opReadPageScanType                  = Opcode(HostCtl<<10 | 0x0046)
	opWritePageScanType                 = Opcode(HostCtl<<10 | 0x0047)
	opReadAFHMode                       = Opcode(HostCtl<<10 | 0x0048)
	opWriteAFHMode                      = Opcode(HostCtl<<10 | 0x0049)
	opReadExtInquiryResponse            = Opcode(HostCtl<<10 | 0x0051)
	opWriteExtInquiryResponse           = Opcode(HostCtl<<10 | 0x0052)
	opRefreshEncryptionKey              = Opcode(HostCtl<<10 | 0x0053)
	opReadSimplePairingMode             = Opcode(HostCtl<<10 | 0x0055)
	opWriteSimplePairingMode            = Opcode(HostCtl<<10 | 0x0056)
	opReadLocalOobData                  = Opcode(HostCtl<<10 | 0x0057)
	opReadInqResponseTransmitPowerLevel = Opcode(HostCtl<<10 | 0x0058)
	opWriteInquiryTransmitPowerLevel    = Opcode(HostCtl<<10 | 0x0059)
	opReadDefaultErrorDataReporting     = Opcode(HostCtl<<10 | 0x005A)
	opWriteDefaultErrorDataReporting    = Opcode(HostCtl<<10 | 0x005B)
	opEnhancedFlush                     = Opcode(HostCtl<<10 | 0x005F)
	opSendKeypressNotify                = Opcode(HostCtl<<10 | 0x0060)
	opReadLogicalLinkAcceptTimeout      = Opcode(HostCtl<<10 | 0x0061)
	opWriteLogicalLinkAcceptTimeout     = Opcode(HostCtl<<10 | 0x0062)
	opSetEventMaskPage2                 = Opcode(HostCtl<<10 | 0x0063)
	opReadLocationData                  = Opcode(HostCtl<<10 | 0x0064)
	opWriteLocationData                 = Opcode(HostCtl<<10 | 0x0065)
	opReadFlowControlMode               = Opcode(HostCtl<<10 | 0x0066)
	opWriteFlowControlMode              = Opcode(HostCtl<<10 | 0x0067)
	opReadEnhancedTransmitpowerLevel    = Opcode(HostCtl<<10 | 0x0068)
	opReadBestEffortFlushTimeout        = Opcode(HostCtl<<10 | 0x0069)
	opWriteBestEffortFlushTimeout       = Opcode(HostCtl<<10 | 0x006A)
	opReadLEHostSupported               = Opcode(HostCtl<<10 | 0x006C)
	opWriteLEHostSupported              = Opcode(HostCtl<<10 | 0x006D)
)

const (
	opLESetEventMask                      = Opcode(LECtl<<10 | 0x0001)
	opLEReadBufferSize                    = Opcode(LECtl<<10 | 0x0002)
	opLEReadLocalSupportedFeatures        = Opcode(LECtl<<10 | 0x0003)
	opLESetRandomAddress                  = Opcode(LECtl<<10 | 0x0005)
	opLESetAdvertisingParameters          = Opcode(LECtl<<10 | 0x0006)
	opLEReadAdvertisingChannelTxPower     = Opcode(LECtl<<10 | 0x0007)
	opLESetAdvertisingData                = Opcode(LECtl<<10 | 0x0008)
	opLESetScanResponseData               = Opcode(LECtl<<10 | 0x0009)
	opLESetAdvertiseEnable                = Opcode(LECtl<<10 | 0x000a)
	opLESetScanParameters                 = Opcode(LECtl<<10 | 0x000b)
	opLESetScanEnable                     = Opcode(LECtl<<10 | 0x000c)
	opLECreateConn                        = Opcode(LECtl<<10 | 0x000d)
	opLECreateConnCancel                  = Opcode(LECtl<<10 | 0x000e)
	opLEReadWhiteListSize                 = Opcode(LECtl<<10 | 0x000f)
	opLEClearWhiteList                    = Opcode(LECtl<<10 | 0x0010)
	opLEAddDeviceToWhiteList              = Opcode(LECtl<<10 | 0x0011)
	opLERemoveDeviceFromWhiteList         = Opcode(LECtl<<10 | 0x0012)
	opLEConnUpdate                        = Opcode(LECtl<<10 | 0x0013)
	opLESetHostChannelClassification      = Opcode(LECtl<<10 | 0x0014)
	opLEReadChannelMap                    = Opcode(LECtl<<10 | 0x0015)
	opLEReadRemoteUsedFeatures            = Opcode(LECtl<<10 | 0x0016)
	opLEEncrypt                           = Opcode(LECtl<<10 | 0x0017)
	opLERand                              = Opcode(LECtl<<10 | 0x0018)
	opLEStartEncryption                   = Opcode(LECtl<<10 | 0x0019)
	opLELTKReply                          = Opcode(LECtl<<10 | 0x001a)
	opLELTKNegReply                       = Opcode(LECtl<<10 | 0x001b)
	opLEReadSupportedStates               = Opcode(LECtl<<10 | 0x001c)
	opLEReceiverTest                      = Opcode(LECtl<<10 | 0x001d)
	opLETransmitterTest                   = Opcode(LECtl<<10 | 0x001e)
	opLETestEnd                           = Opcode(LECtl<<10 | 0x001f)
	opLERemoteConnectionParameterReply    = Opcode(LECtl<<10 | 0x0020)
	opLERemoteConnectionParameterNegReply = Opcode(LECtl<<10 | 0x0021)
)

var opName = map[Opcode]string{

	opInquiry:                "Inquiry",
	opInquiryCancel:          "Inquiry Cancel",
	opPeriodicInquiry:        "Periodic Inquiry Mode",
	opExitPeriodicInquiry:    "Exit Periodic Inquiry Mode",
	opCreateConn:             "Create Connection",
	opDisconnect:             "Disconnect",
	opCreateConnCancel:       "Create Connection Cancel",
	opAcceptConnReq:          "Accept Connection Request",
	opRejectConnReq:          "Reject Connection Request",
	opLinkKeyReply:           "Link Key Request Reply",
	opLinkKeyNegReply:        "Link Key Request Negative Reply",
	opPinCodeReply:           "PIN Code Request Reply",
	opPinCodeNegReply:        "PIN Code Request Negative Reply",
	opSetConnPtype:           "Change Connection Packet Type",
	opAuthRequested:          "Authentication Request",
	opSetConnEncrypt:         "Set Connection Encryption",
	opChangeConnLinkKey:      "Change Connection Link Key",
	opMasterLinkKey:          "Master Link Key",
	opRemoteNameReq:          "Remote Name Request",
	opRemoteNameReqCancel:    "Remote Name Request Cancel",
	opReadRemoteFeatures:     "Read Remote Supported Features",
	opReadRemoteExtFeatures:  "Read Remote Extended Features",
	opReadRemoteVersion:      "Read Remote Version Information",
	opReadClockOffset:        "Read Clock Offset",
	opReadLMPHandle:          "Read LMP Handle",
	opSetupSyncConn:          "Setup Synchronous Connection",
	opAcceptSyncConnReq:      "Aceept Synchronous Connection",
	opRejectSyncConnReq:      "Recject Synchronous Connection",
	opIOCapabilityReply:      "IO Capability Request Reply",
	opUserConfirmReply:       "User Confirmation Request Reply",
	opUserConfirmNegReply:    "User Confirmation Negative Reply",
	opUserPasskeyReply:       "User Passkey Request Reply",
	opUserPasskeyNegReply:    "User Passkey Request Negative Reply",
	opRemoteOOBDataReply:     "Remote OOB Data Request Reply",
	opRemoteOOBDataNegReply:  "Remote OOB Data Request Negative Reply",
	opIOCapabilityNegReply:   "IO Capability Request Negative Reply",
	opCreatePhysicalLink:     "Create Physical Link",
	opAcceptPhysicalLink:     "Accept Physical Link",
	opDisconnectPhysicalLink: "Disconnect Physical Link",
	opCreateLogicalLink:      "Create Logical Link",
	opAcceptLogicalLink:      "Accept Logical Link",
	opDisconnectLogicalLink:  "Disconnect Logical Link",
	opLogicalLinkCancel:      "Logical Link Cancel",
	opFlowSpecModify:         "Flow Spec Modify",

	opHoldMode:               "Hold Mode",
	opSniffMode:              "Sniff Mode",
	opExitSniffMode:          "Exit Sniff Mode",
	opParkMode:               "Park State",
	opExitParkMode:           "Exit Park State",
	opQoSSetup:               "QoS Setup",
	opRoleDiscovery:          "Role Discovery",
	opSwitchRole:             "Switch Role",
	opReadLinkPolicy:         "Read Link Policy Settings",
	opWriteLinkPolicy:        "Write Link Policy Settings",
	opReadDefaultLinkPolicy:  "Read Default Link Policy Settings",
	opWriteDefaultLinkPolicy: "Write Default Link Policy Settings",
	opFlowSpecification:      "Flow Specification",
	opSniffSubrating:         "Sniff Subrating",

	opSetEventMask:                      "Set Event Mask",
	opReset:                             "Reset",
	opSetEventFlt:                       "Set Event Filter",
	opFlush:                             "Flush",
	opReadPinType:                       "Read PIN Type",
	opWritePinType:                      "Write PIN Type",
	opCreateNewUnitKey:                  "Create New Unit Key",
	opReadStoredLinkKey:                 "Read Stored Link Key",
	opWriteStoredLinkKey:                "Write Stored Link Key",
	opDeleteStoredLinkKey:               "Delete Stored Link Key",
	opWriteLocalName:                    "Write Local Name",
	opReadLocalName:                     "Read Local Name",
	opReadConnAcceptTimeout:             "Read Connection Accept Timeout",
	opWriteConnAcceptTimeout:            "Write Connection Accept Timeout",
	opReadPageTimeout:                   "Read Page Timeout",
	opWritePageTimeout:                  "Write Page Timeout",
	opReadScanEnable:                    "Read Scan Enable",
	opWriteScanEnable:                   "Write Scan Enable",
	opReadPageActivity:                  "Read Page Scan Activity",
	opWritePageActivity:                 "Write Page Scan Activity",
	opReadInqActivity:                   "Read Inquiry Scan Activity",
	opWriteInqActivity:                  "Write Inquiry Scan Activity",
	opReadAuthEnable:                    "Read Authentication Enable",
	opWriteAuthEnable:                   "Write Authentication Enable",
	opReadClassOfDev:                    "Read Class of Device",
	opWriteClassOfDevice:                "Write Class of Device",
	opReadVoiceSetting:                  "Read Voice Setting",
	opWriteVoiceSetting:                 "Write Voice Setting",
	opReadAutomaticFlushTimeout:         "Read Automatic Flush Timeout",
	opWriteAutomaticFlushTimeout:        "Write Automatic Flush Timeout",
	opReadNumBroadcastRetrans:           "Read Num Broadcast Retransmissions",
	opWriteNumBroadcastRetrans:          "Write Num Broadcast Retransmissions",
	opReadHoldModeActivity:              "Read Hold Mode Activity",
	opWriteHoldModeActivity:             "Write Hold Mode Activity",
	opReadTransmitPowerLevel:            "Read Transmit Power Level",
	opReadSyncFlowEnable:                "Read Synchronous Flow Control",
	opWriteSyncFlowEnable:               "Write Synchronous Flow Control",
	opSetControllerToHostFC:             "Set Controller To Host Flow Control",
	opHostBufferSize:                    "Host Buffer Size",
	opHostNumCompPkts:                   "Host Number Of Completed Packets",
	opReadLinkSupervisionTimeout:        "Read Link Supervision Timeout",
	opWriteLinkSupervisionTimeout:       "Write Link Supervision Timeout",
	opReadNumSupportedIAC:               "Read Number Of Supported IAC",
	opReadCurrentIACLAP:                 "Read Current IAC LAP",
	opWriteCurrentIACLAP:                "Write Current IAC LAP",
	opSetAFHClassification:              "Set AFH Host Channel Classification",
	opReadInquiryScanType:               "Read Inquiry Scan Type",
	opWriteInquiryScanType:              "Write Inquiry Scan Type",
	opReadInquiryMode:                   "Read Inquiry Mode",
	opWriteInquiryMode:                  "Write Inquiry Mode",
	opReadPageScanType:                  "Read Page Scan Type",
	opWritePageScanType:                 "Write Page Scan Type",
	opReadAFHMode:                       "Read AFH Channel Assessment Mode",
	opWriteAFHMode:                      "Write AFH Channel Assesment Mode",
	opReadExtInquiryResponse:            "Read Extended Inquiry Response",
	opWriteExtInquiryResponse:           "Write Extended Inquiry Response",
	opRefreshEncryptionKey:              "Refresh Encryption Key",
	opReadSimplePairingMode:             "Read Simple Pairing Mode",
	opWriteSimplePairingMode:            "Write Simple Pairing Mode",
	opReadLocalOobData:                  "Read Local OOB Data",
	opReadInqResponseTransmitPowerLevel: "Read Inquiry Response Transmit Power Level",
	opWriteInquiryTransmitPowerLevel:    "Write Inquiry Response Transmit Power Level",
	opReadDefaultErrorDataReporting:     "Read Default Erroneous Data Reporting",
	opWriteDefaultErrorDataReporting:    "Write Default Erroneous Data Reporting",
	opEnhancedFlush:                     "Enhanced Flush",
	opSendKeypressNotify:                "Send Keypress Notification",
	opReadLogicalLinkAcceptTimeout:      "Read Logical Link Accept Timeout",
	opWriteLogicalLinkAcceptTimeout:     "Write Logical Link Accept Timeout",
	opSetEventMaskPage2:                 "Set Event Mask Page 2",
	opReadLocationData:                  "Read Location Data",
	opWriteLocationData:                 "Write Location Data",
	opReadFlowControlMode:               "Read Flow Control Mode",
	opWriteFlowControlMode:              "Write Flow Control Mode",
	opReadEnhancedTransmitpowerLevel:    "Read Enhanced Transmit Power Level",
	opReadBestEffortFlushTimeout:        "Read Best Effort Flush Timeout",
	opWriteBestEffortFlushTimeout:       "Write Best Effort Flush Timeout",
	opReadLEHostSupported:               "Read LE Host Supported",
	opWriteLEHostSupported:              "Write LE Host Supported",

	opLESetEventMask:                      "LE Set Event Mask",
	opLEReadBufferSize:                    "LE Read Buffer Size",
	opLEReadLocalSupportedFeatures:        "LE Read Local Supported Features",
	opLESetRandomAddress:                  "LE Set Random Address",
	opLESetAdvertisingParameters:          "LE Set Advertising Parameters",
	opLEReadAdvertisingChannelTxPower:     "LE Read Advertising Channel Tx Power",
	opLESetAdvertisingData:                "LE Set Advertising Data",
	opLESetScanResponseData:               "LE Set Scan Response Data",
	opLESetAdvertiseEnable:                "LE Set Advertising Enable",
	opLESetScanParameters:                 "LE Set Scan Parameters",
	opLESetScanEnable:                     "LE Set Scan Enable",
	opLECreateConn:                        "LE Create Connection",
	opLECreateConnCancel:                  "LE Create Connection Cancel",
	opLEReadWhiteListSize:                 "LE Read White List Size",
	opLEClearWhiteList:                    "LE Clear White List",
	opLEAddDeviceToWhiteList:              "LE Add Device To White List",
	opLERemoveDeviceFromWhiteList:         "LE Remove Device From White List",
	opLEConnUpdate:                        "LE Connection Update",
	opLESetHostChannelClassification:      "LE Set Host Channel Classification",
	opLEReadChannelMap:                    "LE Read Channel Map",
	opLEReadRemoteUsedFeatures:            "LE Read Remote Used Features",
	opLEEncrypt:                           "LE Encrypt",
	opLERand:                              "LE Rand",
	opLEStartEncryption:                   "LE Star Encryption",
	opLELTKReply:                          "LE Long Term Key Request Reply",
	opLELTKNegReply:                       "LE Long Term Key Request Negative Reply",
	opLEReadSupportedStates:               "LE Read Supported States",
	opLEReceiverTest:                      "LE Reciever Test",
	opLETransmitterTest:                   "LE Transmitter Test",
	opLETestEnd:                           "LE Test End",
	opLERemoteConnectionParameterReply:    "LE Remote Connection Parameter Request Reply",
	opLERemoteConnectionParameterNegReply: "LE Remote Connection Parameter Request Negative Repl",
}

type order struct{ binary.ByteOrder }

var o = order{binary.LittleEndian}

func (o order) PutUint8(b []byte, v uint8) { b[0] = v }
func (o order) PutMac(b []byte, m [6]byte) {
	b[0], b[1], b[2], b[3], b[4], b[5] = m[5], m[4], m[3], m[2], m[1], m[0]
}

// Link Control Commands

// Disconnect (0x0006)
type Disconnect struct {
	ConnectionHandle uint16
	Reason           uint8
}

func (c Disconnect) Marshal() []byte {
	return []byte{byte(c.ConnectionHandle), byte(c.ConnectionHandle >> 8), c.Reason}
}
func (c Disconnect) Opcode() Opcode { return opDisconnect }

// No Return Parameters, Check for Disconnection Complete Event
type DisconnectRP struct{}

// Link Policy Commands

// Write Default Link Policy
type WriteDefaultLinkPolicy struct {
	DefaultLinkPolicySettings uint16
}

func (c WriteDefaultLinkPolicy) Marshal() []byte {
	return []byte{uint8(c.DefaultLinkPolicySettings), uint8(c.DefaultLinkPolicySettings >> 8)}
}
func (c WriteDefaultLinkPolicy) Opcode() Opcode { return opWriteDefaultLinkPolicy }

type WriteDefaultLinkPolicyRP struct {
	Status uint8
}

// Host Control Commands

// Set Event Mask (0x0001)
type SetEventMask struct {
	EventMask uint64
}

func (c SetEventMask) Marshal() []byte {
	b := [8]byte{}
	o.PutUint64(b[0:], c.EventMask)
	return b[:]
}
func (c SetEventMask) Opcode() Opcode { return opSetEventMask }

type SetEventMaskRP struct {
	Status uint8
}

// Reset (0x0002)
type Reset struct{}

func (c Reset) Marshal() []byte { return nil }
func (c Reset) Opcode() Opcode  { return opReset }

type ResetRP struct {
	Status uint8
}

// Set Event Filter (0x0003)
type SetEventFlt struct {
	FilterType          uint8
	FilterConditionType uint8
	Condition           uint8
}

// FIXME: This structures are overloading. Need more effort for decoding
func (c SetEventFlt) Marshal() []byte {
	return []byte{}
}

func (c SetEventFlt) Opcode() Opcode { return opSetEventFlt }

type SetEventFltRP struct {
	Status uint8
}

// Flush (0x0008)
type Flush struct {
	ConnectionHandle uint16
}

func (c Flush) Marshal() []byte {
	return []byte{byte(c.ConnectionHandle), byte(c.ConnectionHandle >> 8)}
}
func (c Flush) Opcode() Opcode { return opFlush }

type FlushRP struct {
	Status uint8
}

// Write Page Timeout (0x0018)
type WritePageTimeout struct {
	PageTimeout uint16
}

func (c WritePageTimeout) Marshal() []byte {
	return []byte{uint8(c.PageTimeout), uint8(c.PageTimeout >> 8)}
}
func (c WritePageTimeout) Opcode() Opcode { return opWritePageTimeout }

type WritePageTimeoutRP struct{}

// Write Class of Device (0x0024)
type WriteClassOfDevice struct {
	ClassOfDevice [3]byte
}

func (c WriteClassOfDevice) Marshal() []byte {
	return []byte{c.ClassOfDevice[0], c.ClassOfDevice[1], c.ClassOfDevice[2]}
}
func (c WriteClassOfDevice) Opcode() Opcode { return opWriteClassOfDevice }

type WriteClassOfDevRP struct {
	Status uint8
}

// Write Host Buffer Size (0x0033)
type HostBufferSize struct {
	HostACLDataPacketLength            uint16
	HostSynchronousDataPacketLength    uint8
	HostTotalNumACLDataPackets         uint16
	HostTotalNumSynchronousDataPackets uint16
}

func (c HostBufferSize) Marshal() []byte {
	b := [7]byte{}
	o.PutUint16(b[0:], c.HostACLDataPacketLength)
	o.PutUint8(b[2:], c.HostSynchronousDataPacketLength)
	o.PutUint16(b[3:], c.HostTotalNumACLDataPackets)
	o.PutUint16(b[5:], c.HostTotalNumSynchronousDataPackets)
	return b[:]
}
func (c HostBufferSize) Opcode() Opcode { return opHostBufferSize }

type HostBufferSizeRP struct {
	Status uint8
}

// Write Inquiry Scan Type (0x0043)
type WriteInquiryScanType struct {
	ScanType uint8
}

func (c WriteInquiryScanType) Marshal() []byte { return []byte{c.ScanType} }
func (c WriteInquiryScanType) Opcode() Opcode  { return opWriteInquiryScanType }

type WriteInquiryScanTypeRP struct {
	Status uint8
}

// Write Inquiry Mode (0x0045)
type WriteInquiryMode struct {
	InquiryMode uint8
}

func (c WriteInquiryMode) Marshal() []byte { return []byte{c.InquiryMode} }
func (c WriteInquiryMode) Opcode() Opcode  { return opWriteInquiryMode }

type WriteInquiryModeRP struct {
	Status uint8
}

// Write Page Scan Type (0x0046)
type WritePageScanType struct {
	PageScanType uint8
}

func (c WritePageScanType) Marshal() []byte { return []byte{c.PageScanType} }
func (c WritePageScanType) Opcode() Opcode  { return opWritePageScanType }

type WritePageScanTypeRP struct {
	Status uint8
}

// Write Simple Pairing Mode (0x0056)
type WriteSimplePairingMode struct {
	SimplePairingMode uint8
}

func (c WriteSimplePairingMode) Marshal() []byte { return []byte{c.SimplePairingMode} }
func (c WriteSimplePairingMode) Opcode() Opcode  { return opWriteSimplePairingMode }

type WriteSimplePairingModeRP struct{}

// Set Event Mask Page 2 (0x0063)
type SetEventMaskPage2 struct {
	EventMaskPage2 uint64
}

func (c SetEventMaskPage2) Marshal() []byte {
	b := [8]byte{}
	o.PutUint64(b[0:], c.EventMaskPage2)
	return b[:]
}
func (c SetEventMaskPage2) Opcode() Opcode { return opSetEventMaskPage2 }

type SetEventMaskPage2RP struct {
	Status uint8
}

// Write LE Host Supported (0x006D)
type WriteLEHostSupported struct {
	LESupportedHost    uint8
	SimultaneousLEHost uint8
}

func (c WriteLEHostSupported) Marshal() []byte { return []byte{c.LESupportedHost, c.SimultaneousLEHost} }
func (c WriteLEHostSupported) Opcode() Opcode  { return opWriteLEHostSupported }

type WriteLeHostSupportedRP struct {
	Status uint8
}

// LE Controller Commands

// LE Set Event Mask (0x0001)
type LESetEventMask struct {
	LEEventMask uint64
}

func (c LESetEventMask) Marshal() []byte {
	b := make([]byte, 8)
	o.PutUint64(b, c.LEEventMask)
	return b
}

func (c LESetEventMask) Opcode() Opcode { return opLESetEventMask }

type LESetEventMaskRP struct {
	Status uint8
}

// LE Read Buffer Size (0x0002)
type LEReadBufferSize struct{}

func (c LEReadBufferSize) Marshal() []byte { return nil }
func (c LEReadBufferSize) Opcode() Opcode  { return opLEReadBufferSize }

type LEReadBufferSizeRP struct {
	Status                     uint8
	HCLEACLDataPacketLength    uint16
	HCTotalNumLEACLDataPackets uint8
}

// LE Read Local Supported Features (0x0003)
type LEReadLocalSupportedFeatures struct{}

func (c LEReadLocalSupportedFeatures) Marshal() []byte { return nil }
func (c LEReadLocalSupportedFeatures) Opcode() Opcode  { return opLEReadLocalSupportedFeatures }

type LEReadLocalSupportedFeaturesRP struct {
	Status     uint8
	LEFeatures uint64
}

type LESetRandomAddress struct {
	RandomAddress [6]byte
}

// LE Set Random Address (0x0005)
func (c LESetRandomAddress) Marshal() []byte {
	b := [6]byte{}
	o.PutMac(b[:], c.RandomAddress)
	return b[:]
}
func (c LESetRandomAddress) Opcode() Opcode { return opLESetRandomAddress }

type LESetRandomAddressRP struct {
	Status uint8
}

// LE Set Advertising Parameters (0x0006)
type LESetAdvertisingParameters struct {
	AdvertisingIntervalMin  uint16
	AdvertisingIntervalMax  uint16
	AdvertisingType         uint8
	OwnAddressType          uint8
	DirectAddressType       uint8
	DirectAddress           [6]byte
	AdvertisingChannelMap   uint8
	AdvertisingFilterPolicy uint8
}

func (c LESetAdvertisingParameters) Marshal() []byte {
	b := [15]byte{}
	o.PutUint16(b[0:], c.AdvertisingIntervalMin)
	o.PutUint16(b[2:], c.AdvertisingIntervalMax)
	o.PutUint8(b[4:], c.AdvertisingType)
	o.PutUint8(b[5:], c.OwnAddressType)
	o.PutUint8(b[6:], c.DirectAddressType)
	o.PutMac(b[7:], c.DirectAddress)
	o.PutUint8(b[13:], c.AdvertisingChannelMap)
	o.PutUint8(b[14:], c.AdvertisingFilterPolicy)
	return b[:]
}
func (c LESetAdvertisingParameters) Opcode() Opcode { return opLESetAdvertisingParameters }

type LESetAdvertisingParametersRP struct {
	Status uint8
}

// LE Read Advertising Channel Tx Power (0x0007)
type LEReadAdvertisingChannelTxPower struct{}

func (c LEReadAdvertisingChannelTxPower) Marshal() []byte { return nil }
func (c LEReadAdvertisingChannelTxPower) Opcode() Opcode  { return opLEReadAdvertisingChannelTxPower }

type LEReadAdvertisingChannelTxPowerRP struct {
	Status             uint8
	TransmitPowerLevel uint8
}

// LE Set Advertising Data (0x0008)
type LESetAdvertisingData struct {
	AdvertisingDataLength uint8
	AdvertisingData       [31]byte
}

func (c LESetAdvertisingData) Marshal() []byte {
	return append([]byte{c.AdvertisingDataLength}, c.AdvertisingData[:]...)
}
func (c LESetAdvertisingData) Opcode() Opcode { return opLESetAdvertisingData }

type LESetAdvertisingDataRP struct {
	Status uint8
}

// LE Set Scan Response Data (0x0009)
type LESetScanResponseData struct {
	ScanResponseDataLength uint8
	ScanResponseData       [31]byte
}

func (c LESetScanResponseData) Marshal() []byte {
	return append([]byte{c.ScanResponseDataLength}, c.ScanResponseData[:]...)
}
func (c LESetScanResponseData) Opcode() Opcode { return opLESetScanResponseData }

type LESetScanResponseDataRP struct {
	Status uint8
}

// LE Set Advertising Enable (0x000A)
type LESetAdvertiseEnable struct {
	AdvertisingEnable uint8
}

func (c LESetAdvertiseEnable) Marshal() []byte { return []byte{c.AdvertisingEnable} }
func (c LESetAdvertiseEnable) Opcode() Opcode  { return opLESetAdvertiseEnable }

type LESetAdvertiseEnableRP struct {
	Status uint8
}

// LE Set Scan Parameters (0x000B)
type LESetScanParameters struct {
	LEScanType           uint8
	LEScanInterval       uint16
	LEScanWindow         uint16
	OwnAddressType       uint8
	ScanningFilterPolicy uint8
}

func (c LESetScanParameters) Marshal() []byte {
	b := [7]byte{}
	o.PutUint8(b[0:], c.LEScanType)
	o.PutUint16(b[1:], c.LEScanInterval)
	o.PutUint16(b[3:], c.LEScanWindow)
	o.PutUint8(b[5:], c.OwnAddressType)
	o.PutUint8(b[6:], c.ScanningFilterPolicy)
	return b[:]
}
func (c LESetScanParameters) Opcode() Opcode { return opLESetScanParameters }

type LESetScanParametersRP struct {
	Status uint8
}

// LE Set Scan Enable (0x000C)
type LESetScanEnable struct {
	LEScanEnable     uint8
	FilterDuplicates uint8
}

func (c LESetScanEnable) Marshal() []byte { return []byte{c.LEScanEnable, c.FilterDuplicates} }
func (c LESetScanEnable) Opcode() Opcode  { return opLESetScanEnable }

type LESetScanEnableRP struct {
	Status uint8
}

// LE Create Connection (0x000D)
type LECreateConn struct {
	LEScanInterval        uint16
	LEScanWindow          uint16
	InitiatorFilterPolicy uint8
	PeerAddressType       uint8
	PeerAddress           [6]byte
	OwnAddressType        uint8
	ConnIntervalMin       uint16
	ConnIntervalMax       uint16
	ConnLatency           uint16
	SupervisionTimeout    uint16
	MinimumCELength       uint16
	MaximumCELength       uint16
}

func (c LECreateConn) Marshal() []byte {
	b := [25]byte{}
	o.PutUint16(b[0:], c.LEScanInterval)
	o.PutUint16(b[2:], c.LEScanWindow)
	o.PutUint8(b[4:], c.InitiatorFilterPolicy)
	o.PutUint8(b[5:], c.PeerAddressType)
	o.PutMac(b[6:], c.PeerAddress)
	o.PutUint8(b[12:], c.OwnAddressType)
	o.PutUint16(b[13:], c.ConnIntervalMin)
	o.PutUint16(b[15:], c.ConnIntervalMax)
	o.PutUint16(b[17:], c.ConnLatency)
	o.PutUint16(b[19:], c.SupervisionTimeout)
	o.PutUint16(b[21:], c.MinimumCELength)
	o.PutUint16(b[23:], c.MaximumCELength)
	return b[:]
}
func (c LECreateConn) Opcode() Opcode { return opLECreateConn }

type LECreateConnRP struct{}

// LE Create Connection Cancel (0x000E)
type LECreateConnCancel struct{}

func (c LECreateConnCancel) Marshal() []byte { return nil }
func (c LECreateConnCancel) Opcode() Opcode  { return opLECreateConnCancel }

type LECreateConnCancelRP struct {
	Status uint8
}

// LE Read White List Size (0x000F)
type LEReadWhiteListSize struct{}

func (c LEReadWhiteListSize) Marshal() []byte { return nil }
func (c LEReadWhiteListSize) Opcode() Opcode  { return opLEReadWhiteListSize }

type LEReadWhiteListSizeRP struct {
	Status        uint8
	WhiteListSize uint8
}

// LE Clear White List (0x0010)
type LEClearWhiteList struct{}

func (c LEClearWhiteList) Marshal() []byte { return nil }
func (c LEClearWhiteList) Opcode() Opcode  { return opLEClearWhiteList }

type LEClearWhiteListRP struct {
	Status uint8
}

// LE Add Device To White List (0x0011)
type LEAddDeviceToWhiteList struct {
	AddressType uint8
	Address     [6]byte
}

func (c LEAddDeviceToWhiteList) Marshal() []byte {
	b := [7]byte{}
	o.PutUint8(b[0:], c.AddressType)
	o.PutMac(b[1:], c.Address)
	return b[:]
}
func (c LEAddDeviceToWhiteList) Opcode() Opcode { return opLEAddDeviceToWhiteList }

type LEAddDeviceToWhiteListRP struct {
	Status uint8
}

// LE Remove Device From White List (0x0012)
type LERemoveDeviceFromWhiteList struct {
	AddressType uint8
	Address     [6]byte
}

func (c LERemoveDeviceFromWhiteList) Marshal() []byte {
	b := [7]byte{}
	o.PutUint8(b[0:], c.AddressType)
	o.PutMac(b[1:], c.Address)
	return b[:]
}
func (c LERemoveDeviceFromWhiteList) Opcode() Opcode { return opLERemoveDeviceFromWhiteList }

type LERemoveDeviceFromWhiteListRP struct {
	Status uint8
}

// LE Connection Update (0x0013)
type LEConnUpdate struct {
	ConnectionHandle   uint16
	ConnIntervalMin    uint16
	ConnIntervalMax    uint16
	ConnLatency        uint16
	SupervisionTimeout uint16
	MinimumCELength    uint16
	MaximumCELength    uint16
}

func (c LEConnUpdate) Marshal() []byte {
	b := [14]byte{}
	o.PutUint16(b[0:], c.ConnectionHandle)
	o.PutUint16(b[2:], c.ConnIntervalMin)
	o.PutUint16(b[4:], c.ConnIntervalMax)
	o.PutUint16(b[6:], c.ConnLatency)
	o.PutUint16(b[8:], c.SupervisionTimeout)
	o.PutUint16(b[10:], c.MinimumCELength)
	o.PutUint16(b[12:], c.MaximumCELength)
	return b[:]
}

func (c LEConnUpdate) Opcode() Opcode { return opLEConnUpdate }

type LEConnUpdateRP struct{}

// LE Set Host Channel Classification (0x0014)
type LESetHostChannelClassification struct {
	ChannelMap [5]byte
}

func (c LESetHostChannelClassification) Marshal() []byte { return c.ChannelMap[:] }
func (c LESetHostChannelClassification) Opcode() Opcode  { return opLESetHostChannelClassification }

type LESetHostChannelClassificationRP struct {
	Status uint8
}

// LE Read Channel Map (0x0015)
type LEReadChannelMap struct {
	ConnectionHandle uint16
}

func (c LEReadChannelMap) Marshal() []byte {
	return []byte{uint8(c.ConnectionHandle), uint8(c.ConnectionHandle >> 8)}
}
func (c LEReadChannelMap) Opcode() Opcode { return opLEReadChannelMap }

type LEReadChannelMapRP struct {
	Status           uint8
	ConnectionHandle uint16
	ChannelMap       [5]byte
}

// LE Read Remote Used Features (0x0016)
type LEReadRemoteUsedFeatures struct {
	ConnectionHandle uint16
}

func (c LEReadRemoteUsedFeatures) Marshal() []byte {
	return []byte{uint8(c.ConnectionHandle), uint8(c.ConnectionHandle >> 8)}
}
func (c LEReadRemoteUsedFeatures) Opcode() Opcode { return opLEReadRemoteUsedFeatures }

type LEReadRemoteUsedFeaturesRP struct{}

// LE Encrypt (0x0017)
type LEEncrypt struct {
	Key           [16]byte
	PlaintextData [16]byte
}

func (c LEEncrypt) Marshal() []byte { return append(c.Key[:], c.PlaintextData[:]...) }
func (c LEEncrypt) Opcode() Opcode  { return opLEEncrypt }

type LEEncryptRP struct {
	Stauts        uint8
	EncryptedData [16]byte
}

// LE Rand (0x0018)
type LERand struct{}

func (c LERand) Marshal() []byte { return nil }
func (c LERand) Opcode() Opcode  { return opLERand }

type LERandRP struct {
	Status       uint8
	RandomNumber uint64
}

// LE Start Encryption (0x0019)
type LEStartEncryption struct {
	ConnectionHandle     uint16
	RandomNumber         uint64
	EncryptedDiversifier uint16
	LongTermKey          [16]byte
}

func (c LEStartEncryption) Marshal() []byte {
	b := [12]byte{}
	o.PutUint16(b[0:], c.ConnectionHandle)
	o.PutUint64(b[2:], c.RandomNumber)
	o.PutUint16(b[10:], c.EncryptedDiversifier)
	return append(b[:], c.LongTermKey[:]...)
}

func (c LEStartEncryption) Opcode() Opcode { return opLEStartEncryption }

type LEStartEncryptionRP struct{}

// LE Long Term Key Reply (0x001A)
type LELTKReply struct {
	ConnectionHandle uint16
	LongTermKey      [16]byte
}

func (c LELTKReply) Marshal() []byte {
	return append([]byte{uint8(c.ConnectionHandle), uint8(c.ConnectionHandle >> 8)}, c.LongTermKey[:]...)
}
func (c LELTKReply) Opcode() Opcode { return opLELTKReply }

type LELTKReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}

// LE Long Term Key  Negative Reply (0x001B)
type LELTKNegReply struct {
	ConnectionHandle uint16
}

func (c LELTKNegReply) Marshal() []byte {
	return []byte{uint8(c.ConnectionHandle), uint8(c.ConnectionHandle >> 8)}
}
func (c LELTKNegReply) Opcode() Opcode { return opLELTKNegReply }

type LELTKNegReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}

// LE Read Supported States (0x001C)
type LEReadSupportedStates struct{}

func (c LEReadSupportedStates) Marshal() []byte { return nil }
func (c LEReadSupportedStates) Opcode() Opcode  { return opLEReadSupportedStates }

type LEReadSupportedStatesRP struct {
	Status   uint8
	LEStates [8]byte
}

// LE Reciever Test (0x001D)
type LEReceiverTest struct {
	RXChannel uint8
}

func (c LEReceiverTest) Marshal() []byte { return []byte{c.RXChannel} }
func (c LEReceiverTest) Opcode() Opcode  { return opLEReceiverTest }

type LEReceiverTestRP struct {
	Status uint8
}

// LE Transmitter Test (0x001E)
type LETransmitterTest struct {
	TXChannel        uint8
	LengthOfTestData uint8
	PacketPayload    uint8
}

func (c LETransmitterTest) Marshal() []byte {
	return []byte{c.TXChannel, c.LengthOfTestData, c.PacketPayload}
}
func (c LETransmitterTest) Opcode() Opcode { return opLETransmitterTest }

type LETransmitterTestRP struct {
	Status uint8
}

// LE Test End (0x001F)
type LETestEnd struct{}

func (c LETestEnd) Marshal() []byte { return nil }
func (c LETestEnd) Opcode() Opcode  { return opLETestEnd }

type LETestEndRP struct {
	Status          uint8
	NumberOfPackets uint16
}

// LE Remote Connection Parameters Reply (0x0020)
type LERemoteConnectionParameterReply struct {
	ConnectionHandle uint16
	IntervalMin      uint16
	IntervalMax      uint16
	Latency          uint16
	Timeout          uint16
	MinimumCELength  uint16
	MaximumCELength  uint16
}

func (c LERemoteConnectionParameterReply) Marshal() []byte {
	b := [14]byte{}
	o.PutUint16(b[0:], c.ConnectionHandle)
	o.PutUint16(b[2:], c.IntervalMin)
	o.PutUint16(b[4:], c.IntervalMax)
	o.PutUint16(b[6:], c.Latency)
	o.PutUint16(b[8:], c.Timeout)
	o.PutUint16(b[10:], c.MinimumCELength)
	o.PutUint16(b[12:], c.MaximumCELength)
	return b[:]
}
func (c LERemoteConnectionParameterReply) Opcode() Opcode { return opLERemoteConnectionParameterReply }

type LERemoteConnectionParameterReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}

// LE Remote Connection Parameters Negative Reply (0x0021)
type LERemoteConnectionParameterNegReply struct {
	ConnectionHandle uint16
	Reason           uint8
}

func (c LERemoteConnectionParameterNegReply) Marshal() []byte {
	return []byte{uint8(c.ConnectionHandle), uint8(c.ConnectionHandle >> 8), c.Reason}
}
func (c LERemoteConnectionParameterNegReply) Opcode() Opcode {
	return opLERemoteConnectionParameterNegReply
}

type LERemoteConnectionParameterNegReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}
