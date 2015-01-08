package cmd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/paypal/gatt/linux/internal/event"
	"github.com/paypal/gatt/linux/internal/hci"
)

type CmdParam interface {
	Marshal([]byte)
	Opcode() Opcode
	Len() int
}

func NewCmd(d io.Writer) *Cmd {
	c := &Cmd{
		dev:     d,
		sent:    []*cmdPkt{},
		compc:   make(chan event.CommandCompleteEP),
		statusc: make(chan event.CommandStatusEP),
	}
	go c.processCmdEvents()
	return c
}

type cmdPkt struct {
	op   Opcode
	cp   CmdParam
	done chan []byte
}

func (c cmdPkt) marshal() []byte {
	b := make([]byte, 1+2+1+c.cp.Len())
	b[0] = byte(hci.TypCommandPkt)
	b[1], b[2] = byte(c.op), byte(c.op>>8)
	b[3] = byte(c.cp.Len())
	c.cp.Marshal(b[4:])
	return b
}

type Cmd struct {
	dev     io.Writer
	sent    []*cmdPkt
	compc   chan event.CommandCompleteEP
	statusc chan event.CommandStatusEP
}

func (c Cmd) trace(fmt string, v ...interface{}) {}

func (c *Cmd) HandleComplete(b []byte) error {
	var ep event.CommandCompleteEP
	if err := ep.Unmarshal(b); err != nil {
		return err
	}
	c.compc <- ep
	return nil
}

func (c *Cmd) HandleStatus(b []byte) error {
	var ep event.CommandStatusEP
	if err := ep.Unmarshal(b); err != nil {
		return err
	}
	c.statusc <- ep
	return nil
}

func (c *Cmd) Send(cp CmdParam) ([]byte, error) {
	op := cp.Opcode()
	p := &cmdPkt{op: op, cp: cp, done: make(chan []byte)}
	raw := p.marshal()

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
	linkCtl     = 0x01
	linkPolicy  = 0x02
	hostCtl     = 0x03
	infoParam   = 0x04
	statusParam = 0x05
	testingCmd  = 0X3E
	leCtl       = 0x08
	vendorCmd   = 0X3F
)

type Opcode uint16

func (op Opcode) ogf() uint8     { return uint8((uint16(op) & 0xFC00) >> 10) }
func (op Opcode) ocf() uint16    { return uint16(op) & 0x03FF }
func (op Opcode) String() string { return opName[op] }

const (
	opInquiry                = Opcode(linkCtl<<10 | 0x0001)
	opInquiryCancel          = Opcode(linkCtl<<10 | 0x0002)
	opPeriodicInquiry        = Opcode(linkCtl<<10 | 0x0003)
	opExitPeriodicInquiry    = Opcode(linkCtl<<10 | 0x0004)
	opCreateConn             = Opcode(linkCtl<<10 | 0x0005)
	opDisconnect             = Opcode(linkCtl<<10 | 0x0006)
	opCreateConnCancel       = Opcode(linkCtl<<10 | 0x0008)
	opAcceptConnReq          = Opcode(linkCtl<<10 | 0x0009)
	opRejectConnReq          = Opcode(linkCtl<<10 | 0x000A)
	opLinkKeyReply           = Opcode(linkCtl<<10 | 0x000B)
	opLinkKeyNegReply        = Opcode(linkCtl<<10 | 0x000C)
	opPinCodeReply           = Opcode(linkCtl<<10 | 0x000D)
	opPinCodeNegReply        = Opcode(linkCtl<<10 | 0x000E)
	opSetConnPtype           = Opcode(linkCtl<<10 | 0x000F)
	opAuthRequested          = Opcode(linkCtl<<10 | 0x0011)
	opSetConnEncrypt         = Opcode(linkCtl<<10 | 0x0013)
	opChangeConnLinkKey      = Opcode(linkCtl<<10 | 0x0015)
	opMasterLinkKey          = Opcode(linkCtl<<10 | 0x0017)
	opRemoteNameReq          = Opcode(linkCtl<<10 | 0x0019)
	opRemoteNameReqCancel    = Opcode(linkCtl<<10 | 0x001A)
	opReadRemoteFeatures     = Opcode(linkCtl<<10 | 0x001B)
	opReadRemoteExtFeatures  = Opcode(linkCtl<<10 | 0x001C)
	opReadRemoteVersion      = Opcode(linkCtl<<10 | 0x001D)
	opReadClockOffset        = Opcode(linkCtl<<10 | 0x001F)
	opReadLMPHandle          = Opcode(linkCtl<<10 | 0x0020)
	opSetupSyncConn          = Opcode(linkCtl<<10 | 0x0028)
	opAcceptSyncConnReq      = Opcode(linkCtl<<10 | 0x0029)
	opRejectSyncConnReq      = Opcode(linkCtl<<10 | 0x002A)
	opIOCapabilityReply      = Opcode(linkCtl<<10 | 0x002B)
	opUserConfirmReply       = Opcode(linkCtl<<10 | 0x002C)
	opUserConfirmNegReply    = Opcode(linkCtl<<10 | 0x002D)
	opUserPasskeyReply       = Opcode(linkCtl<<10 | 0x002E)
	opUserPasskeyNegReply    = Opcode(linkCtl<<10 | 0x002F)
	opRemoteOOBDataReply     = Opcode(linkCtl<<10 | 0x0030)
	opRemoteOOBDataNegReply  = Opcode(linkCtl<<10 | 0x0033)
	opIOCapabilityNegReply   = Opcode(linkCtl<<10 | 0x0034)
	opCreatePhysicalLink     = Opcode(linkCtl<<10 | 0x0035)
	opAcceptPhysicalLink     = Opcode(linkCtl<<10 | 0x0036)
	opDisconnectPhysicalLink = Opcode(linkCtl<<10 | 0x0037)
	opCreateLogicalLink      = Opcode(linkCtl<<10 | 0x0038)
	opAcceptLogicalLink      = Opcode(linkCtl<<10 | 0x0039)
	opDisconnectLogicalLink  = Opcode(linkCtl<<10 | 0x003A)
	opLogicalLinkCancel      = Opcode(linkCtl<<10 | 0x003B)
	opFlowSpecModify         = Opcode(linkCtl<<10 | 0x003C)
)

const (
	opHoldMode               = Opcode(linkPolicy<<10 | 0x0001)
	opSniffMode              = Opcode(linkPolicy<<10 | 0x0003)
	opExitSniffMode          = Opcode(linkPolicy<<10 | 0x0004)
	opParkMode               = Opcode(linkPolicy<<10 | 0x0005)
	opExitParkMode           = Opcode(linkPolicy<<10 | 0x0006)
	opQoSSetup               = Opcode(linkPolicy<<10 | 0x0007)
	opRoleDiscovery          = Opcode(linkPolicy<<10 | 0x0009)
	opSwitchRole             = Opcode(linkPolicy<<10 | 0x000B)
	opReadLinkPolicy         = Opcode(linkPolicy<<10 | 0x000C)
	opWriteLinkPolicy        = Opcode(linkPolicy<<10 | 0x000D)
	opReadDefaultLinkPolicy  = Opcode(linkPolicy<<10 | 0x000E)
	opWriteDefaultLinkPolicy = Opcode(linkPolicy<<10 | 0x000F)
	opFlowSpecification      = Opcode(linkPolicy<<10 | 0x0010)
	opSniffSubrating         = Opcode(linkPolicy<<10 | 0x0011)
)
const (
	opSetEventMask                      = Opcode(hostCtl<<10 | 0x0001)
	opReset                             = Opcode(hostCtl<<10 | 0x0003)
	opSetEventFlt                       = Opcode(hostCtl<<10 | 0x0005)
	opFlush                             = Opcode(hostCtl<<10 | 0x0008)
	opReadPinType                       = Opcode(hostCtl<<10 | 0x0009)
	opWritePinType                      = Opcode(hostCtl<<10 | 0x000A)
	opCreateNewUnitKey                  = Opcode(hostCtl<<10 | 0x000B)
	opReadStoredLinkKey                 = Opcode(hostCtl<<10 | 0x000D)
	opWriteStoredLinkKey                = Opcode(hostCtl<<10 | 0x0011)
	opDeleteStoredLinkKey               = Opcode(hostCtl<<10 | 0x0012)
	opWriteLocalName                    = Opcode(hostCtl<<10 | 0x0013)
	opReadLocalName                     = Opcode(hostCtl<<10 | 0x0014)
	opReadConnAcceptTimeout             = Opcode(hostCtl<<10 | 0x0015)
	opWriteConnAcceptTimeout            = Opcode(hostCtl<<10 | 0x0016)
	opReadPageTimeout                   = Opcode(hostCtl<<10 | 0x0017)
	opWritePageTimeout                  = Opcode(hostCtl<<10 | 0x0018)
	opReadScanEnable                    = Opcode(hostCtl<<10 | 0x0019)
	opWriteScanEnable                   = Opcode(hostCtl<<10 | 0x001A)
	opReadPageActivity                  = Opcode(hostCtl<<10 | 0x001B)
	opWritePageActivity                 = Opcode(hostCtl<<10 | 0x001C)
	opReadInqActivity                   = Opcode(hostCtl<<10 | 0x001D)
	opWriteInqActivity                  = Opcode(hostCtl<<10 | 0x001E)
	opReadAuthEnable                    = Opcode(hostCtl<<10 | 0x001F)
	opWriteAuthEnable                   = Opcode(hostCtl<<10 | 0x0020)
	opReadEncryptMode                   = Opcode(hostCtl<<10 | 0x0021)
	opWriteEncryptMode                  = Opcode(hostCtl<<10 | 0x0022)
	opReadClassOfDev                    = Opcode(hostCtl<<10 | 0x0023)
	opWriteClassOfDevice                = Opcode(hostCtl<<10 | 0x0024)
	opReadVoiceSetting                  = Opcode(hostCtl<<10 | 0x0025)
	opWriteVoiceSetting                 = Opcode(hostCtl<<10 | 0x0026)
	opReadAutomaticFlushTimeout         = Opcode(hostCtl<<10 | 0x0027)
	opWriteAutomaticFlushTimeout        = Opcode(hostCtl<<10 | 0x0028)
	opReadNumBroadcastRetrans           = Opcode(hostCtl<<10 | 0x0029)
	opWriteNumBroadcastRetrans          = Opcode(hostCtl<<10 | 0x002A)
	opReadHoldModeActivity              = Opcode(hostCtl<<10 | 0x002B)
	opWriteHoldModeActivity             = Opcode(hostCtl<<10 | 0x002C)
	opReadTransmitPowerLevel            = Opcode(hostCtl<<10 | 0x002D)
	opReadSyncFlowEnable                = Opcode(hostCtl<<10 | 0x002E)
	opWriteSyncFlowEnable               = Opcode(hostCtl<<10 | 0x002F)
	opSetControllerToHostFC             = Opcode(hostCtl<<10 | 0x0031)
	opHostBufferSize                    = Opcode(hostCtl<<10 | 0x0033)
	opHostNumCompPkts                   = Opcode(hostCtl<<10 | 0x0035)
	opReadLinkSupervisionTimeout        = Opcode(hostCtl<<10 | 0x0036)
	opWriteLinkSupervisionTimeout       = Opcode(hostCtl<<10 | 0x0037)
	opReadNumSupportedIAC               = Opcode(hostCtl<<10 | 0x0038)
	opReadCurrentIACLAP                 = Opcode(hostCtl<<10 | 0x0039)
	opWriteCurrentIACLAP                = Opcode(hostCtl<<10 | 0x003A)
	opReadPageScanPeriodMode            = Opcode(hostCtl<<10 | 0x003B)
	opWritePageScanPeriodMode           = Opcode(hostCtl<<10 | 0x003C)
	opReadPageScanMode                  = Opcode(hostCtl<<10 | 0x003D)
	opWritePageScanMode                 = Opcode(hostCtl<<10 | 0x003E)
	opSetAFHClassification              = Opcode(hostCtl<<10 | 0x003F)
	opReadInquiryScanType               = Opcode(hostCtl<<10 | 0x0042)
	opWriteInquiryScanType              = Opcode(hostCtl<<10 | 0x0043)
	opReadInquiryMode                   = Opcode(hostCtl<<10 | 0x0044)
	opWriteInquiryMode                  = Opcode(hostCtl<<10 | 0x0045)
	opReadPageScanType                  = Opcode(hostCtl<<10 | 0x0046)
	opWritePageScanType                 = Opcode(hostCtl<<10 | 0x0047)
	opReadAFHMode                       = Opcode(hostCtl<<10 | 0x0048)
	opWriteAFHMode                      = Opcode(hostCtl<<10 | 0x0049)
	opReadExtInquiryResponse            = Opcode(hostCtl<<10 | 0x0051)
	opWriteExtInquiryResponse           = Opcode(hostCtl<<10 | 0x0052)
	opRefreshEncryptionKey              = Opcode(hostCtl<<10 | 0x0053)
	opReadSimplePairingMode             = Opcode(hostCtl<<10 | 0x0055)
	opWriteSimplePairingMode            = Opcode(hostCtl<<10 | 0x0056)
	opReadLocalOobData                  = Opcode(hostCtl<<10 | 0x0057)
	opReadInqResponseTransmitPowerLevel = Opcode(hostCtl<<10 | 0x0058)
	opWriteInquiryTransmitPowerLevel    = Opcode(hostCtl<<10 | 0x0059)
	opReadDefaultErrorDataReporting     = Opcode(hostCtl<<10 | 0x005A)
	opWriteDefaultErrorDataReporting    = Opcode(hostCtl<<10 | 0x005B)
	opEnhancedFlush                     = Opcode(hostCtl<<10 | 0x005F)
	opSendKeypressNotify                = Opcode(hostCtl<<10 | 0x0060)
	opReadLogicalLinkAcceptTimeout      = Opcode(hostCtl<<10 | 0x0061)
	opWriteLogicalLinkAcceptTimeout     = Opcode(hostCtl<<10 | 0x0062)
	opSetEventMaskPage2                 = Opcode(hostCtl<<10 | 0x0063)
	opReadLocationData                  = Opcode(hostCtl<<10 | 0x0064)
	opWriteLocationData                 = Opcode(hostCtl<<10 | 0x0065)
	opReadFlowControlMode               = Opcode(hostCtl<<10 | 0x0066)
	opWriteFlowControlMode              = Opcode(hostCtl<<10 | 0x0067)
	opReadEnhancedTransmitpowerLevel    = Opcode(hostCtl<<10 | 0x0068)
	opReadBestEffortFlushTimeout        = Opcode(hostCtl<<10 | 0x0069)
	opWriteBestEffortFlushTimeout       = Opcode(hostCtl<<10 | 0x006A)
	opReadLEHostSupported               = Opcode(hostCtl<<10 | 0x006C)
	opWriteLEHostSupported              = Opcode(hostCtl<<10 | 0x006D)
)

const (
	opLESetEventMask                      = Opcode(leCtl<<10 | 0x0001)
	opLEReadBufferSize                    = Opcode(leCtl<<10 | 0x0002)
	opLEReadLocalSupportedFeatures        = Opcode(leCtl<<10 | 0x0003)
	opLESetRandomAddress                  = Opcode(leCtl<<10 | 0x0005)
	opLESetAdvertisingParameters          = Opcode(leCtl<<10 | 0x0006)
	opLEReadAdvertisingChannelTxPower     = Opcode(leCtl<<10 | 0x0007)
	opLESetAdvertisingData                = Opcode(leCtl<<10 | 0x0008)
	opLESetScanResponseData               = Opcode(leCtl<<10 | 0x0009)
	opLESetAdvertiseEnable                = Opcode(leCtl<<10 | 0x000a)
	opLESetScanParameters                 = Opcode(leCtl<<10 | 0x000b)
	opLESetScanEnable                     = Opcode(leCtl<<10 | 0x000c)
	opLECreateConn                        = Opcode(leCtl<<10 | 0x000d)
	opLECreateConnCancel                  = Opcode(leCtl<<10 | 0x000e)
	opLEReadWhiteListSize                 = Opcode(leCtl<<10 | 0x000f)
	opLEClearWhiteList                    = Opcode(leCtl<<10 | 0x0010)
	opLEAddDeviceToWhiteList              = Opcode(leCtl<<10 | 0x0011)
	opLERemoveDeviceFromWhiteList         = Opcode(leCtl<<10 | 0x0012)
	opLEConnUpdate                        = Opcode(leCtl<<10 | 0x0013)
	opLESetHostChannelClassification      = Opcode(leCtl<<10 | 0x0014)
	opLEReadChannelMap                    = Opcode(leCtl<<10 | 0x0015)
	opLEReadRemoteUsedFeatures            = Opcode(leCtl<<10 | 0x0016)
	opLEEncrypt                           = Opcode(leCtl<<10 | 0x0017)
	opLERand                              = Opcode(leCtl<<10 | 0x0018)
	opLEStartEncryption                   = Opcode(leCtl<<10 | 0x0019)
	opLELTKReply                          = Opcode(leCtl<<10 | 0x001a)
	opLELTKNegReply                       = Opcode(leCtl<<10 | 0x001b)
	opLEReadSupportedStates               = Opcode(leCtl<<10 | 0x001c)
	opLEReceiverTest                      = Opcode(leCtl<<10 | 0x001d)
	opLETransmitterTest                   = Opcode(leCtl<<10 | 0x001e)
	opLETestEnd                           = Opcode(leCtl<<10 | 0x001f)
	opLERemoteConnectionParameterReply    = Opcode(leCtl<<10 | 0x0020)
	opLERemoteConnectionParameterNegReply = Opcode(leCtl<<10 | 0x0021)
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
func (o order) PutMAC(b []byte, m [6]byte) {
	b[0], b[1], b[2], b[3], b[4], b[5] = m[5], m[4], m[3], m[2], m[1], m[0]
}

// Link Control Commands

// Disconnect (0x0006)
type Disconnect struct {
	ConnectionHandle uint16
	Reason           uint8
}

func (c Disconnect) Opcode() Opcode { return opDisconnect }
func (c Disconnect) Len() int       { return 3 }
func (c Disconnect) Marshal(b []byte) {
	o.PutUint16(b[0:], c.ConnectionHandle)
	b[2] = c.Reason
}

// No Return Parameters, Check for Disconnection Complete Event
type DisconnectRP struct{}

// Link Policy Commands

// Write Default Link Policy
type WriteDefaultLinkPolicy struct{ DefaultLinkPolicySettings uint16 }

func (c WriteDefaultLinkPolicy) Opcode() Opcode   { return opWriteDefaultLinkPolicy }
func (c WriteDefaultLinkPolicy) Len() int         { return 2 }
func (c WriteDefaultLinkPolicy) Marshal(b []byte) { o.PutUint16(b, c.DefaultLinkPolicySettings) }

type WriteDefaultLinkPolicyRP struct{ Status uint8 }

// Host Control Commands

// Set Event Mask (0x0001)
type SetEventMask struct{ EventMask uint64 }

func (c SetEventMask) Opcode() Opcode   { return opSetEventMask }
func (c SetEventMask) Len() int         { return 8 }
func (c SetEventMask) Marshal(b []byte) { o.PutUint64(b, c.EventMask) }

type SetEventMaskRP struct{ Status uint8 }

// Reset (0x0002)
type Reset struct{}

func (c Reset) Opcode() Opcode   { return opReset }
func (c Reset) Len() int         { return 0 }
func (c Reset) Marshal(b []byte) {}

type ResetRP struct{ Status uint8 }

// Set Event Filter (0x0003)
// FIXME: This structures are overloading.
// Both Marshal() and Len() are just placeholder.
// Need more effort for decoding.
// type SetEventFlt struct {
// 	FilterType          uint8
// 	FilterConditionType uint8
// 	Condition           uint8
// }

// func (c SetEventFlt) Opcode() Opcode   { return opSetEventFlt }
// func (c SetEventFlt) Len() int         { return 0 }
// func (c SetEventFlt) Marshal(b []byte) {}

type SetEventFltRP struct{ Status uint8 }

// Flush (0x0008)
type Flush struct{ ConnectionHandle uint16 }

func (c Flush) Opcode() Opcode   { return opFlush }
func (c Flush) Len() int         { return 2 }
func (c Flush) Marshal(b []byte) { o.PutUint16(b, c.ConnectionHandle) }

type FlushRP struct{ Status uint8 }

// Write Page Timeout (0x0018)
type WritePageTimeout struct{ PageTimeout uint16 }

func (c WritePageTimeout) Opcode() Opcode   { return opWritePageTimeout }
func (c WritePageTimeout) Len() int         { return 2 }
func (c WritePageTimeout) Marshal(b []byte) { o.PutUint16(b, c.PageTimeout) }

type WritePageTimeoutRP struct{}

// Write Class of Device (0x0024)
type WriteClassOfDevice struct{ ClassOfDevice [3]byte }

func (c WriteClassOfDevice) Opcode() Opcode   { return opWriteClassOfDevice }
func (c WriteClassOfDevice) Len() int         { return 3 }
func (c WriteClassOfDevice) Marshal(b []byte) { copy(b, c.ClassOfDevice[:]) }

type WriteClassOfDevRP struct{ Status uint8 }

// Write Host Buffer Size (0x0033)
type HostBufferSize struct {
	HostACLDataPacketLength            uint16
	HostSynchronousDataPacketLength    uint8
	HostTotalNumACLDataPackets         uint16
	HostTotalNumSynchronousDataPackets uint16
}

func (c HostBufferSize) Opcode() Opcode { return opHostBufferSize }
func (c HostBufferSize) Len() int       { return 7 }
func (c HostBufferSize) Marshal(b []byte) {
	o.PutUint16(b[0:], c.HostACLDataPacketLength)
	o.PutUint8(b[2:], c.HostSynchronousDataPacketLength)
	o.PutUint16(b[3:], c.HostTotalNumACLDataPackets)
	o.PutUint16(b[5:], c.HostTotalNumSynchronousDataPackets)
}

type HostBufferSizeRP struct{ Status uint8 }

// Write Inquiry Scan Type (0x0043)
type WriteInquiryScanType struct{ ScanType uint8 }

func (c WriteInquiryScanType) Opcode() Opcode   { return opWriteInquiryScanType }
func (c WriteInquiryScanType) Len() int         { return 1 }
func (c WriteInquiryScanType) Marshal(b []byte) { b[0] = c.ScanType }

type WriteInquiryScanTypeRP struct{ Status uint8 }

// Write Inquiry Mode (0x0045)
type WriteInquiryMode struct {
	InquiryMode uint8
}

func (c WriteInquiryMode) Opcode() Opcode   { return opWriteInquiryMode }
func (c WriteInquiryMode) Len() int         { return 1 }
func (c WriteInquiryMode) Marshal(b []byte) { b[0] = c.InquiryMode }

type WriteInquiryModeRP struct{ Status uint8 }

// Write Page Scan Type (0x0046)
type WritePageScanType struct{ PageScanType uint8 }

func (c WritePageScanType) Opcode() Opcode   { return opWritePageScanType }
func (c WritePageScanType) Len() int         { return 1 }
func (c WritePageScanType) Marshal(b []byte) { b[0] = c.PageScanType }

type WritePageScanTypeRP struct{ Status uint8 }

// Write Simple Pairing Mode (0x0056)
type WriteSimplePairingMode struct{ SimplePairingMode uint8 }

func (c WriteSimplePairingMode) Opcode() Opcode   { return opWriteSimplePairingMode }
func (c WriteSimplePairingMode) Len() int         { return 1 }
func (c WriteSimplePairingMode) Marshal(b []byte) { b[0] = c.SimplePairingMode }

type WriteSimplePairingModeRP struct{}

// Set Event Mask Page 2 (0x0063)
type SetEventMaskPage2 struct{ EventMaskPage2 uint64 }

func (c SetEventMaskPage2) Opcode() Opcode   { return opSetEventMaskPage2 }
func (c SetEventMaskPage2) Len() int         { return 8 }
func (c SetEventMaskPage2) Marshal(b []byte) { o.PutUint64(b, c.EventMaskPage2) }

type SetEventMaskPage2RP struct{ Status uint8 }

// Write LE Host Supported (0x006D)
type WriteLEHostSupported struct {
	LESupportedHost    uint8
	SimultaneousLEHost uint8
}

func (c WriteLEHostSupported) Opcode() Opcode   { return opWriteLEHostSupported }
func (c WriteLEHostSupported) Len() int         { return 2 }
func (c WriteLEHostSupported) Marshal(b []byte) { b[0], b[1] = c.LESupportedHost, c.SimultaneousLEHost }

type WriteLeHostSupportedRP struct{ Status uint8 }

// LE Controller Commands

// LE Set Event Mask (0x0001)
type LESetEventMask struct{ LEEventMask uint64 }

func (c LESetEventMask) Opcode() Opcode   { return opLESetEventMask }
func (c LESetEventMask) Len() int         { return 8 }
func (c LESetEventMask) Marshal(b []byte) { o.PutUint64(b, c.LEEventMask) }

type LESetEventMaskRP struct {
	Status uint8
}

// LE Read Buffer Size (0x0002)
type LEReadBufferSize struct{}

func (c LEReadBufferSize) Opcode() Opcode   { return opLEReadBufferSize }
func (c LEReadBufferSize) Len() int         { return 1 }
func (c LEReadBufferSize) Marshal(b []byte) {}

type LEReadBufferSizeRP struct {
	Status                     uint8
	HCLEACLDataPacketLength    uint16
	HCTotalNumLEACLDataPackets uint8
}

// LE Read Local Supported Features (0x0003)
type LEReadLocalSupportedFeatures struct{}

func (c LEReadLocalSupportedFeatures) Opcode() Opcode   { return opLEReadLocalSupportedFeatures }
func (c LEReadLocalSupportedFeatures) Len() int         { return 0 }
func (c LEReadLocalSupportedFeatures) Marshal(b []byte) {}

type LEReadLocalSupportedFeaturesRP struct {
	Status     uint8
	LEFeatures uint64
}

// LE Set Random Address (0x0005)
type LESetRandomAddress struct{ RandomAddress [6]byte }

func (c LESetRandomAddress) Opcode() Opcode   { return opLESetRandomAddress }
func (c LESetRandomAddress) Len() int         { return 6 }
func (c LESetRandomAddress) Marshal(b []byte) { o.PutMAC(b, c.RandomAddress) }

type LESetRandomAddressRP struct{ Status uint8 }

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

func (c LESetAdvertisingParameters) Opcode() Opcode { return opLESetAdvertisingParameters }
func (c LESetAdvertisingParameters) Len() int       { return 15 }
func (c LESetAdvertisingParameters) Marshal(b []byte) {
	o.PutUint16(b[0:], c.AdvertisingIntervalMin)
	o.PutUint16(b[2:], c.AdvertisingIntervalMax)
	o.PutUint8(b[4:], c.AdvertisingType)
	o.PutUint8(b[5:], c.OwnAddressType)
	o.PutUint8(b[6:], c.DirectAddressType)
	o.PutMAC(b[7:], c.DirectAddress)
	o.PutUint8(b[13:], c.AdvertisingChannelMap)
	o.PutUint8(b[14:], c.AdvertisingFilterPolicy)
}

type LESetAdvertisingParametersRP struct{ Status uint8 }

// LE Read Advertising Channel Tx Power (0x0007)
type LEReadAdvertisingChannelTxPower struct{}

func (c LEReadAdvertisingChannelTxPower) Opcode() Opcode   { return opLEReadAdvertisingChannelTxPower }
func (c LEReadAdvertisingChannelTxPower) Len() int         { return 0 }
func (c LEReadAdvertisingChannelTxPower) Marshal(b []byte) {}

type LEReadAdvertisingChannelTxPowerRP struct {
	Status             uint8
	TransmitPowerLevel uint8
}

// LE Set Advertising Data (0x0008)
type LESetAdvertisingData struct {
	AdvertisingDataLength uint8
	AdvertisingData       [31]byte
}

func (c LESetAdvertisingData) Opcode() Opcode { return opLESetAdvertisingData }
func (c LESetAdvertisingData) Len() int       { return 32 }
func (c LESetAdvertisingData) Marshal(b []byte) {
	b[0] = c.AdvertisingDataLength
	copy(b[1:], c.AdvertisingData[:c.AdvertisingDataLength])
}

type LESetAdvertisingDataRP struct{ Status uint8 }

// LE Set Scan Response Data (0x0009)
type LESetScanResponseData struct {
	ScanResponseDataLength uint8
	ScanResponseData       [31]byte
}

func (c LESetScanResponseData) Opcode() Opcode { return opLESetScanResponseData }
func (c LESetScanResponseData) Len() int       { return 32 }
func (c LESetScanResponseData) Marshal(b []byte) {
	b[0] = c.ScanResponseDataLength
	copy(b[1:], c.ScanResponseData[:c.ScanResponseDataLength])
}

type LESetScanResponseDataRP struct{ Status uint8 }

// LE Set Advertising Enable (0x000A)
type LESetAdvertiseEnable struct{ AdvertisingEnable uint8 }

func (c LESetAdvertiseEnable) Opcode() Opcode   { return opLESetAdvertiseEnable }
func (c LESetAdvertiseEnable) Len() int         { return 1 }
func (c LESetAdvertiseEnable) Marshal(b []byte) { b[0] = c.AdvertisingEnable }

type LESetAdvertiseEnableRP struct{ Status uint8 }

// LE Set Scan Parameters (0x000B)
type LESetScanParameters struct {
	LEScanType           uint8
	LEScanInterval       uint16
	LEScanWindow         uint16
	OwnAddressType       uint8
	ScanningFilterPolicy uint8
}

func (c LESetScanParameters) Opcode() Opcode { return opLESetScanParameters }
func (c LESetScanParameters) Len() int       { return 7 }
func (c LESetScanParameters) Marshal(b []byte) {
	o.PutUint8(b[0:], c.LEScanType)
	o.PutUint16(b[1:], c.LEScanInterval)
	o.PutUint16(b[3:], c.LEScanWindow)
	o.PutUint8(b[5:], c.OwnAddressType)
	o.PutUint8(b[6:], c.ScanningFilterPolicy)
}

type LESetScanParametersRP struct{ Status uint8 }

// LE Set Scan Enable (0x000C)
type LESetScanEnable struct {
	LEScanEnable     uint8
	FilterDuplicates uint8
}

func (c LESetScanEnable) Opcode() Opcode   { return opLESetScanEnable }
func (c LESetScanEnable) Len() int         { return 2 }
func (c LESetScanEnable) Marshal(b []byte) { b[0], b[1] = c.LEScanEnable, c.FilterDuplicates }

type LESetScanEnableRP struct{ Status uint8 }

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

func (c LECreateConn) Opcode() Opcode { return opLECreateConn }
func (c LECreateConn) Len() int       { return 25 }
func (c LECreateConn) Marshal(b []byte) {
	o.PutUint16(b[0:], c.LEScanInterval)
	o.PutUint16(b[2:], c.LEScanWindow)
	o.PutUint8(b[4:], c.InitiatorFilterPolicy)
	o.PutUint8(b[5:], c.PeerAddressType)
	o.PutMAC(b[6:], c.PeerAddress)
	o.PutUint8(b[12:], c.OwnAddressType)
	o.PutUint16(b[13:], c.ConnIntervalMin)
	o.PutUint16(b[15:], c.ConnIntervalMax)
	o.PutUint16(b[17:], c.ConnLatency)
	o.PutUint16(b[19:], c.SupervisionTimeout)
	o.PutUint16(b[21:], c.MinimumCELength)
	o.PutUint16(b[23:], c.MaximumCELength)
}

type LECreateConnRP struct{}

// LE Create Connection Cancel (0x000E)
type LECreateConnCancel struct{}

func (c LECreateConnCancel) Opcode() Opcode   { return opLECreateConnCancel }
func (c LECreateConnCancel) Len() int         { return 0 }
func (c LECreateConnCancel) Marshal(b []byte) {}

type LECreateConnCancelRP struct{ Status uint8 }

// LE Read White List Size (0x000F)
type LEReadWhiteListSize struct{}

func (c LEReadWhiteListSize) Opcode() Opcode   { return opLEReadWhiteListSize }
func (c LEReadWhiteListSize) Len() int         { return 0 }
func (c LEReadWhiteListSize) Marshal(b []byte) {}

type LEReadWhiteListSizeRP struct {
	Status        uint8
	WhiteListSize uint8
}

// LE Clear White List (0x0010)
type LEClearWhiteList struct{}

func (c LEClearWhiteList) Opcode() Opcode   { return opLEClearWhiteList }
func (c LEClearWhiteList) Len() int         { return 0 }
func (c LEClearWhiteList) Marshal(b []byte) {}

type LEClearWhiteListRP struct{ Status uint8 }

// LE Add Device To White List (0x0011)
type LEAddDeviceToWhiteList struct {
	AddressType uint8
	Address     [6]byte
}

func (c LEAddDeviceToWhiteList) Opcode() Opcode { return opLEAddDeviceToWhiteList }
func (c LEAddDeviceToWhiteList) Len() int       { return 7 }
func (c LEAddDeviceToWhiteList) Marshal(b []byte) {
	b[0] = c.AddressType
	o.PutMAC(b[1:], c.Address)
}

type LEAddDeviceToWhiteListRP struct{ Status uint8 }

// LE Remove Device From White List (0x0012)
type LERemoveDeviceFromWhiteList struct {
	AddressType uint8
	Address     [6]byte
}

func (c LERemoveDeviceFromWhiteList) Opcode() Opcode { return opLERemoveDeviceFromWhiteList }
func (c LERemoveDeviceFromWhiteList) Len() int       { return 7 }
func (c LERemoveDeviceFromWhiteList) Marshal(b []byte) {
	b[0] = c.AddressType
	o.PutMAC(b[1:], c.Address)
}

type LERemoveDeviceFromWhiteListRP struct{ Status uint8 }

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

func (c LEConnUpdate) Opcode() Opcode { return opLEConnUpdate }
func (c LEConnUpdate) Len() int       { return 14 }
func (c LEConnUpdate) Marshal(b []byte) {
	o.PutUint16(b[0:], c.ConnectionHandle)
	o.PutUint16(b[2:], c.ConnIntervalMin)
	o.PutUint16(b[4:], c.ConnIntervalMax)
	o.PutUint16(b[6:], c.ConnLatency)
	o.PutUint16(b[8:], c.SupervisionTimeout)
	o.PutUint16(b[10:], c.MinimumCELength)
	o.PutUint16(b[12:], c.MaximumCELength)
}

type LEConnUpdateRP struct{}

// LE Set Host Channel Classification (0x0014)
type LESetHostChannelClassification struct{ ChannelMap [5]byte }

func (c LESetHostChannelClassification) Opcode() Opcode   { return opLESetHostChannelClassification }
func (c LESetHostChannelClassification) Len() int         { return 5 }
func (c LESetHostChannelClassification) Marshal(b []byte) { copy(b, c.ChannelMap[:]) }

type LESetHostChannelClassificationRP struct{ Status uint8 }

// LE Read Channel Map (0x0015)
type LEReadChannelMap struct{ ConnectionHandle uint16 }

func (c LEReadChannelMap) Opcode() Opcode   { return opLEReadChannelMap }
func (c LEReadChannelMap) Len() int         { return 2 }
func (c LEReadChannelMap) Marshal(b []byte) { o.PutUint16(b, c.ConnectionHandle) }

type LEReadChannelMapRP struct {
	Status           uint8
	ConnectionHandle uint16
	ChannelMap       [5]byte
}

// LE Read Remote Used Features (0x0016)
type LEReadRemoteUsedFeatures struct{ ConnectionHandle uint16 }

func (c LEReadRemoteUsedFeatures) Opcode() Opcode   { return opLEReadRemoteUsedFeatures }
func (c LEReadRemoteUsedFeatures) Len() int         { return 8 }
func (c LEReadRemoteUsedFeatures) Marshal(b []byte) { o.PutUint16(b, c.ConnectionHandle) }

type LEReadRemoteUsedFeaturesRP struct{}

// LE Encrypt (0x0017)
type LEEncrypt struct {
	Key           [16]byte
	PlaintextData [16]byte
}

func (c LEEncrypt) Opcode() Opcode { return opLEEncrypt }
func (c LEEncrypt) Len() int       { return 32 }
func (c LEEncrypt) Marshal(b []byte) {
	copy(b[0:], c.Key[:])
	copy(b[16:], c.PlaintextData[:])
}

type LEEncryptRP struct {
	Stauts        uint8
	EncryptedData [16]byte
}

// LE Rand (0x0018)
type LERand struct{}

func (c LERand) Opcode() Opcode   { return opLERand }
func (c LERand) Len() int         { return 0 }
func (c LERand) Marshal(b []byte) {}

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

func (c LEStartEncryption) Opcode() Opcode { return opLEStartEncryption }
func (c LEStartEncryption) Len() int       { return 28 }
func (c LEStartEncryption) Marshal(b []byte) {
	o.PutUint16(b[0:], c.ConnectionHandle)
	o.PutUint64(b[2:], c.RandomNumber)
	o.PutUint16(b[10:], c.EncryptedDiversifier)
	copy(b[12:], c.LongTermKey[:])
}

type LEStartEncryptionRP struct{}

// LE Long Term Key Reply (0x001A)
type LELTKReply struct {
	ConnectionHandle uint16
	LongTermKey      [16]byte
}

func (c LELTKReply) Opcode() Opcode { return opLELTKReply }
func (c LELTKReply) Len() int       { return 18 }
func (c LELTKReply) Marshal(b []byte) {
	o.PutUint16(b[0:], c.ConnectionHandle)
	copy(b[2:], c.LongTermKey[:])
}

type LELTKReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}

// LE Long Term Key  Negative Reply (0x001B)
type LELTKNegReply struct{ ConnectionHandle uint16 }

func (c LELTKNegReply) Opcode() Opcode   { return opLELTKNegReply }
func (c LELTKNegReply) Len() int         { return 2 }
func (c LELTKNegReply) Marshal(b []byte) { o.PutUint16(b, c.ConnectionHandle) }

type LELTKNegReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}

// LE Read Supported States (0x001C)
type LEReadSupportedStates struct{}

func (c LEReadSupportedStates) Opcode() Opcode   { return opLEReadSupportedStates }
func (c LEReadSupportedStates) Len() int         { return 0 }
func (c LEReadSupportedStates) Marshal(b []byte) {}

type LEReadSupportedStatesRP struct {
	Status   uint8
	LEStates [8]byte
}

// LE Reciever Test (0x001D)
type LEReceiverTest struct{ RXChannel uint8 }

func (c LEReceiverTest) Opcode() Opcode   { return opLEReceiverTest }
func (c LEReceiverTest) Len() int         { return 1 }
func (c LEReceiverTest) Marshal(b []byte) { b[0] = c.RXChannel }

type LEReceiverTestRP struct{ Status uint8 }

// LE Transmitter Test (0x001E)
type LETransmitterTest struct {
	TXChannel        uint8
	LengthOfTestData uint8
	PacketPayload    uint8
}

func (c LETransmitterTest) Opcode() Opcode { return opLETransmitterTest }
func (c LETransmitterTest) Len() int       { return 3 }
func (c LETransmitterTest) Marshal(b []byte) {
	b[0], b[1], b[2] = c.TXChannel, c.LengthOfTestData, c.PacketPayload
}

type LETransmitterTestRP struct{ Status uint8 }

// LE Test End (0x001F)
type LETestEnd struct{}

func (c LETestEnd) Opcode() Opcode   { return opLETestEnd }
func (c LETestEnd) Len() int         { return 0 }
func (c LETestEnd) Marshal(b []byte) {}

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

func (c LERemoteConnectionParameterReply) Opcode() Opcode { return opLERemoteConnectionParameterReply }
func (c LERemoteConnectionParameterReply) Len() int       { return 14 }
func (c LERemoteConnectionParameterReply) Marshal(b []byte) {
	o.PutUint16(b[0:], c.ConnectionHandle)
	o.PutUint16(b[2:], c.IntervalMin)
	o.PutUint16(b[4:], c.IntervalMax)
	o.PutUint16(b[6:], c.Latency)
	o.PutUint16(b[8:], c.Timeout)
	o.PutUint16(b[10:], c.MinimumCELength)
	o.PutUint16(b[12:], c.MaximumCELength)
}

type LERemoteConnectionParameterReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}

// LE Remote Connection Parameters Negative Reply (0x0021)
type LERemoteConnectionParameterNegReply struct {
	ConnectionHandle uint16
	Reason           uint8
}

func (c LERemoteConnectionParameterNegReply) Opcode() Opcode {
	return opLERemoteConnectionParameterNegReply
}
func (c LERemoteConnectionParameterNegReply) Len() int { return 3 }
func (c LERemoteConnectionParameterNegReply) Marshal(b []byte) {
	o.PutUint16(b[0:], c.ConnectionHandle)
	b[2] = c.Reason
}

type LERemoteConnectionParameterNegReplyRP struct {
	Status           uint8
	ConnectionHandle uint16
}
