package linux

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
)

type cmdParam interface {
	marshal([]byte)
	opcode() opcode
	len() int
}

func newCmd(d io.Writer) *cmd {
	c := &cmd{
		dev:     d,
		sent:    []*cmdPkt{},
		compc:   make(chan commandCompleteEP),
		statusc: make(chan commandStatusEP),
	}
	go c.processCmdEvents()
	return c
}

type cmdPkt struct {
	op   opcode
	cp   cmdParam
	done chan []byte
}

func (c cmdPkt) marshal() []byte {
	b := make([]byte, 1+2+1+c.cp.len())
	b[0] = byte(typCommandPkt)
	b[1], b[2] = byte(c.op), byte(c.op>>8)
	b[3] = byte(c.cp.len())
	c.cp.marshal(b[4:])
	return b
}

type cmd struct {
	dev     io.Writer
	sent    []*cmdPkt
	compc   chan commandCompleteEP
	statusc chan commandStatusEP
}

func (c cmd) trace(fmt string, v ...interface{}) {}

func (c *cmd) handleComplete(b []byte) error {
	var ep commandCompleteEP
	if err := ep.unmarshal(b); err != nil {
		return err
	}
	c.compc <- ep
	return nil
}

func (c *cmd) handleStatus(b []byte) error {
	var ep commandStatusEP
	if err := ep.unmarshal(b); err != nil {
		return err
	}
	c.statusc <- ep
	return nil
}

func (c *cmd) send(cp cmdParam) ([]byte, error) {
	op := cp.opcode()
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

func (c *cmd) sendAndCheckResp(cp cmdParam, exp []byte) error {
	rsp, err := c.send(cp)
	if err != nil {
		return err
	}
	// Don't care about the response
	if len(exp) == 0 {
		return nil
	}
	// Check the if status is one of the expected value
	if !bytes.Contains(exp, rsp[0:1]) {
		return fmt.Errorf("HCI command: '%s' return 0x%02X, expect: [%X] ", cp.opcode(), rsp[0], exp)
	}
	return nil
}

func (c *cmd) processCmdEvents() {
	for {
		select {
		case status := <-c.statusc:
			found := false
			for i, p := range c.sent {
				if uint16(p.op) == status.commandOpcode {
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
				if uint16(p.op) == comp.commandOPCode {
					found = true
					c.sent = append(c.sent[:i], c.sent[i+1:]...)
					p.done <- comp.returnParameters
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

type opcode uint16

func (op opcode) ogf() uint8     { return uint8((uint16(op) & 0xFC00) >> 10) }
func (op opcode) ocf() uint16    { return uint16(op) & 0x03FF }
func (op opcode) String() string { return opName[op] }

const (
	opInquiry                = opcode(linkCtl<<10 | 0x0001)
	opInquiryCancel          = opcode(linkCtl<<10 | 0x0002)
	opPeriodicInquiry        = opcode(linkCtl<<10 | 0x0003)
	opExitPeriodicInquiry    = opcode(linkCtl<<10 | 0x0004)
	opCreateConn             = opcode(linkCtl<<10 | 0x0005)
	opDisconnect             = opcode(linkCtl<<10 | 0x0006)
	opCreateConnCancel       = opcode(linkCtl<<10 | 0x0008)
	opAcceptConnReq          = opcode(linkCtl<<10 | 0x0009)
	opRejectConnReq          = opcode(linkCtl<<10 | 0x000A)
	opLinkKeyReply           = opcode(linkCtl<<10 | 0x000B)
	opLinkKeyNegReply        = opcode(linkCtl<<10 | 0x000C)
	opPinCodeReply           = opcode(linkCtl<<10 | 0x000D)
	opPinCodeNegReply        = opcode(linkCtl<<10 | 0x000E)
	opSetConnPtype           = opcode(linkCtl<<10 | 0x000F)
	opAuthRequested          = opcode(linkCtl<<10 | 0x0011)
	opSetConnEncrypt         = opcode(linkCtl<<10 | 0x0013)
	opChangeConnLinkKey      = opcode(linkCtl<<10 | 0x0015)
	opMasterLinkKey          = opcode(linkCtl<<10 | 0x0017)
	opRemoteNameReq          = opcode(linkCtl<<10 | 0x0019)
	opRemoteNameReqCancel    = opcode(linkCtl<<10 | 0x001A)
	opReadRemoteFeatures     = opcode(linkCtl<<10 | 0x001B)
	opReadRemoteExtFeatures  = opcode(linkCtl<<10 | 0x001C)
	opReadRemoteVersion      = opcode(linkCtl<<10 | 0x001D)
	opReadClockOffset        = opcode(linkCtl<<10 | 0x001F)
	opReadLMPHandle          = opcode(linkCtl<<10 | 0x0020)
	opSetupSyncConn          = opcode(linkCtl<<10 | 0x0028)
	opAcceptSyncConnReq      = opcode(linkCtl<<10 | 0x0029)
	opRejectSyncConnReq      = opcode(linkCtl<<10 | 0x002A)
	opIOCapabilityReply      = opcode(linkCtl<<10 | 0x002B)
	opUserConfirmReply       = opcode(linkCtl<<10 | 0x002C)
	opUserConfirmNegReply    = opcode(linkCtl<<10 | 0x002D)
	opUserPasskeyReply       = opcode(linkCtl<<10 | 0x002E)
	opUserPasskeyNegReply    = opcode(linkCtl<<10 | 0x002F)
	opRemoteOOBDataReply     = opcode(linkCtl<<10 | 0x0030)
	opRemoteOOBDataNegReply  = opcode(linkCtl<<10 | 0x0033)
	opIOCapabilityNegReply   = opcode(linkCtl<<10 | 0x0034)
	opCreatePhysicalLink     = opcode(linkCtl<<10 | 0x0035)
	opAcceptPhysicalLink     = opcode(linkCtl<<10 | 0x0036)
	opDisconnectPhysicalLink = opcode(linkCtl<<10 | 0x0037)
	opCreateLogicalLink      = opcode(linkCtl<<10 | 0x0038)
	opAcceptLogicalLink      = opcode(linkCtl<<10 | 0x0039)
	opDisconnectLogicalLink  = opcode(linkCtl<<10 | 0x003A)
	opLogicalLinkCancel      = opcode(linkCtl<<10 | 0x003B)
	opFlowSpecModify         = opcode(linkCtl<<10 | 0x003C)
)

const (
	opHoldMode               = opcode(linkPolicy<<10 | 0x0001)
	opSniffMode              = opcode(linkPolicy<<10 | 0x0003)
	opExitSniffMode          = opcode(linkPolicy<<10 | 0x0004)
	opParkMode               = opcode(linkPolicy<<10 | 0x0005)
	opExitParkMode           = opcode(linkPolicy<<10 | 0x0006)
	opQoSSetup               = opcode(linkPolicy<<10 | 0x0007)
	opRoleDiscovery          = opcode(linkPolicy<<10 | 0x0009)
	opSwitchRole             = opcode(linkPolicy<<10 | 0x000B)
	opReadLinkPolicy         = opcode(linkPolicy<<10 | 0x000C)
	opWriteLinkPolicy        = opcode(linkPolicy<<10 | 0x000D)
	opReadDefaultLinkPolicy  = opcode(linkPolicy<<10 | 0x000E)
	opWriteDefaultLinkPolicy = opcode(linkPolicy<<10 | 0x000F)
	opFlowSpecification      = opcode(linkPolicy<<10 | 0x0010)
	opSniffSubrating         = opcode(linkPolicy<<10 | 0x0011)
)
const (
	opSetEventMask                      = opcode(hostCtl<<10 | 0x0001)
	opReset                             = opcode(hostCtl<<10 | 0x0003)
	opSetEventFlt                       = opcode(hostCtl<<10 | 0x0005)
	opFlush                             = opcode(hostCtl<<10 | 0x0008)
	opReadPinType                       = opcode(hostCtl<<10 | 0x0009)
	opWritePinType                      = opcode(hostCtl<<10 | 0x000A)
	opCreateNewUnitKey                  = opcode(hostCtl<<10 | 0x000B)
	opReadStoredLinkKey                 = opcode(hostCtl<<10 | 0x000D)
	opWriteStoredLinkKey                = opcode(hostCtl<<10 | 0x0011)
	opDeleteStoredLinkKey               = opcode(hostCtl<<10 | 0x0012)
	opWriteLocalName                    = opcode(hostCtl<<10 | 0x0013)
	opReadLocalName                     = opcode(hostCtl<<10 | 0x0014)
	opReadConnAcceptTimeout             = opcode(hostCtl<<10 | 0x0015)
	opWriteConnAcceptTimeout            = opcode(hostCtl<<10 | 0x0016)
	opReadPageTimeout                   = opcode(hostCtl<<10 | 0x0017)
	opWritePageTimeout                  = opcode(hostCtl<<10 | 0x0018)
	opReadScanEnable                    = opcode(hostCtl<<10 | 0x0019)
	opWriteScanEnable                   = opcode(hostCtl<<10 | 0x001A)
	opReadPageActivity                  = opcode(hostCtl<<10 | 0x001B)
	opWritePageActivity                 = opcode(hostCtl<<10 | 0x001C)
	opReadInqActivity                   = opcode(hostCtl<<10 | 0x001D)
	opWriteInqActivity                  = opcode(hostCtl<<10 | 0x001E)
	opReadAuthEnable                    = opcode(hostCtl<<10 | 0x001F)
	opWriteAuthEnable                   = opcode(hostCtl<<10 | 0x0020)
	opReadEncryptMode                   = opcode(hostCtl<<10 | 0x0021)
	opWriteEncryptMode                  = opcode(hostCtl<<10 | 0x0022)
	opReadClassOfDev                    = opcode(hostCtl<<10 | 0x0023)
	opWriteClassOfDevice                = opcode(hostCtl<<10 | 0x0024)
	opReadVoiceSetting                  = opcode(hostCtl<<10 | 0x0025)
	opWriteVoiceSetting                 = opcode(hostCtl<<10 | 0x0026)
	opReadAutomaticFlushTimeout         = opcode(hostCtl<<10 | 0x0027)
	opWriteAutomaticFlushTimeout        = opcode(hostCtl<<10 | 0x0028)
	opReadNumBroadcastRetrans           = opcode(hostCtl<<10 | 0x0029)
	opWriteNumBroadcastRetrans          = opcode(hostCtl<<10 | 0x002A)
	opReadHoldModeActivity              = opcode(hostCtl<<10 | 0x002B)
	opWriteHoldModeActivity             = opcode(hostCtl<<10 | 0x002C)
	opReadTransmitPowerLevel            = opcode(hostCtl<<10 | 0x002D)
	opReadSyncFlowEnable                = opcode(hostCtl<<10 | 0x002E)
	opWriteSyncFlowEnable               = opcode(hostCtl<<10 | 0x002F)
	opSetControllerToHostFC             = opcode(hostCtl<<10 | 0x0031)
	opHostBufferSize                    = opcode(hostCtl<<10 | 0x0033)
	opHostNumCompPkts                   = opcode(hostCtl<<10 | 0x0035)
	opReadLinkSupervisionTimeout        = opcode(hostCtl<<10 | 0x0036)
	opWriteLinkSupervisionTimeout       = opcode(hostCtl<<10 | 0x0037)
	opReadNumSupportedIAC               = opcode(hostCtl<<10 | 0x0038)
	opReadCurrentIACLAP                 = opcode(hostCtl<<10 | 0x0039)
	opWriteCurrentIACLAP                = opcode(hostCtl<<10 | 0x003A)
	opReadPageScanPeriodMode            = opcode(hostCtl<<10 | 0x003B)
	opWritePageScanPeriodMode           = opcode(hostCtl<<10 | 0x003C)
	opReadPageScanMode                  = opcode(hostCtl<<10 | 0x003D)
	opWritePageScanMode                 = opcode(hostCtl<<10 | 0x003E)
	opSetAFHClassification              = opcode(hostCtl<<10 | 0x003F)
	opReadInquiryScanType               = opcode(hostCtl<<10 | 0x0042)
	opWriteInquiryScanType              = opcode(hostCtl<<10 | 0x0043)
	opReadInquiryMode                   = opcode(hostCtl<<10 | 0x0044)
	opWriteInquiryMode                  = opcode(hostCtl<<10 | 0x0045)
	opReadPageScanType                  = opcode(hostCtl<<10 | 0x0046)
	opWritePageScanType                 = opcode(hostCtl<<10 | 0x0047)
	opReadAFHMode                       = opcode(hostCtl<<10 | 0x0048)
	opWriteAFHMode                      = opcode(hostCtl<<10 | 0x0049)
	opReadExtInquiryResponse            = opcode(hostCtl<<10 | 0x0051)
	opWriteExtInquiryResponse           = opcode(hostCtl<<10 | 0x0052)
	opRefreshEncryptionKey              = opcode(hostCtl<<10 | 0x0053)
	opReadSimplePairingMode             = opcode(hostCtl<<10 | 0x0055)
	opWriteSimplePairingMode            = opcode(hostCtl<<10 | 0x0056)
	opReadLocalOobData                  = opcode(hostCtl<<10 | 0x0057)
	opReadInqResponseTransmitPowerLevel = opcode(hostCtl<<10 | 0x0058)
	opWriteInquiryTransmitPowerLevel    = opcode(hostCtl<<10 | 0x0059)
	opReadDefaultErrorDataReporting     = opcode(hostCtl<<10 | 0x005A)
	opWriteDefaultErrorDataReporting    = opcode(hostCtl<<10 | 0x005B)
	opEnhancedFlush                     = opcode(hostCtl<<10 | 0x005F)
	opSendKeypressNotify                = opcode(hostCtl<<10 | 0x0060)
	opReadLogicalLinkAcceptTimeout      = opcode(hostCtl<<10 | 0x0061)
	opWriteLogicalLinkAcceptTimeout     = opcode(hostCtl<<10 | 0x0062)
	opSetEventMaskPage2                 = opcode(hostCtl<<10 | 0x0063)
	opReadLocationData                  = opcode(hostCtl<<10 | 0x0064)
	opWriteLocationData                 = opcode(hostCtl<<10 | 0x0065)
	opReadFlowControlMode               = opcode(hostCtl<<10 | 0x0066)
	opWriteFlowControlMode              = opcode(hostCtl<<10 | 0x0067)
	opReadEnhancedTransmitpowerLevel    = opcode(hostCtl<<10 | 0x0068)
	opReadBestEffortFlushTimeout        = opcode(hostCtl<<10 | 0x0069)
	opWriteBestEffortFlushTimeout       = opcode(hostCtl<<10 | 0x006A)
	opReadLEHostSupported               = opcode(hostCtl<<10 | 0x006C)
	opWriteLEHostSupported              = opcode(hostCtl<<10 | 0x006D)
)

const (
	opLESetEventMask                      = opcode(leCtl<<10 | 0x0001)
	opLEReadBufferSize                    = opcode(leCtl<<10 | 0x0002)
	opLEReadLocalSupportedFeatures        = opcode(leCtl<<10 | 0x0003)
	opLESetRandomAddress                  = opcode(leCtl<<10 | 0x0005)
	opLESetAdvertisingParameters          = opcode(leCtl<<10 | 0x0006)
	opLEReadAdvertisingChannelTxPower     = opcode(leCtl<<10 | 0x0007)
	opLESetAdvertisingData                = opcode(leCtl<<10 | 0x0008)
	opLESetScanResponseData               = opcode(leCtl<<10 | 0x0009)
	opLESetAdvertiseEnable                = opcode(leCtl<<10 | 0x000a)
	opLESetScanParameters                 = opcode(leCtl<<10 | 0x000b)
	opLESetScanEnable                     = opcode(leCtl<<10 | 0x000c)
	opLECreateConn                        = opcode(leCtl<<10 | 0x000d)
	opLECreateConnCancel                  = opcode(leCtl<<10 | 0x000e)
	opLEReadWhiteListSize                 = opcode(leCtl<<10 | 0x000f)
	opLEClearWhiteList                    = opcode(leCtl<<10 | 0x0010)
	opLEAddDeviceToWhiteList              = opcode(leCtl<<10 | 0x0011)
	opLERemoveDeviceFromWhiteList         = opcode(leCtl<<10 | 0x0012)
	opLEConnUpdate                        = opcode(leCtl<<10 | 0x0013)
	opLESetHostChannelClassification      = opcode(leCtl<<10 | 0x0014)
	opLEReadChannelMap                    = opcode(leCtl<<10 | 0x0015)
	opLEReadRemoteUsedFeatures            = opcode(leCtl<<10 | 0x0016)
	opLEEncrypt                           = opcode(leCtl<<10 | 0x0017)
	opLERand                              = opcode(leCtl<<10 | 0x0018)
	opLEStartEncryption                   = opcode(leCtl<<10 | 0x0019)
	opLELTKReply                          = opcode(leCtl<<10 | 0x001a)
	opLELTKNegReply                       = opcode(leCtl<<10 | 0x001b)
	opLEReadSupportedStates               = opcode(leCtl<<10 | 0x001c)
	opLEReceiverTest                      = opcode(leCtl<<10 | 0x001d)
	opLETransmitterTest                   = opcode(leCtl<<10 | 0x001e)
	opLETestEnd                           = opcode(leCtl<<10 | 0x001f)
	opLERemoteConnectionParameterReply    = opcode(leCtl<<10 | 0x0020)
	opLERemoteConnectionParameterNegReply = opcode(leCtl<<10 | 0x0021)
)

var opName = map[opcode]string{

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
	opSendKeypressNotify:                "send Keypress Notification",
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

// Link Control Commands

// Disconnect (0x0006)
type disconnect struct {
	connectionHandle uint16
	reason           uint8
}

func (c disconnect) opcode() opcode { return opDisconnect }
func (c disconnect) len() int       { return 3 }
func (c disconnect) marshal(b []byte) {
	o.PutUint16(b[0:], c.connectionHandle)
	b[2] = c.reason
}

// No Return Parameters, Check for Disconnection Complete Event
type disconnectRP struct{}

// Link Policy Commands

// Write Default Link Policy
type writeDefaultLinkPolicy struct{ defaultLinkPolicySettings uint16 }

func (c writeDefaultLinkPolicy) opcode() opcode   { return opWriteDefaultLinkPolicy }
func (c writeDefaultLinkPolicy) len() int         { return 2 }
func (c writeDefaultLinkPolicy) marshal(b []byte) { o.PutUint16(b, c.defaultLinkPolicySettings) }

type writeDefaultLinkPolicyRP struct{ status uint8 }

// Host Control Commands

// Set Event Mask (0x0001)
type setEventMask struct{ eventMask uint64 }

func (c setEventMask) opcode() opcode   { return opSetEventMask }
func (c setEventMask) len() int         { return 8 }
func (c setEventMask) marshal(b []byte) { o.PutUint64(b, c.eventMask) }

type setEventMaskRP struct{ status uint8 }

// Reset (0x0002)
type reset struct{}

func (c reset) opcode() opcode   { return opReset }
func (c reset) len() int         { return 0 }
func (c reset) marshal(b []byte) {}

type resetRP struct{ status uint8 }

// Set Event Filter (0x0003)
// FIXME: This structures are overloading.
// Both marshal() and len() are just placeholder.
// Need more effort for decoding.
// type SetEventFlt struct {
// 	filterType          uint8
// 	filterConditionType uint8
// 	condition           uint8
// }

// func (c setEventFlt) opcode() opcode   { return opSetEventFlt }
// func (c setEventFlt) len() int         { return 0 }
// func (c setEventFlt) marshal(b []byte) {}

type setEventFltRP struct{ status uint8 }

// Flush (0x0008)
type flush struct{ connectionHandle uint16 }

func (c flush) opcode() opcode   { return opFlush }
func (c flush) len() int         { return 2 }
func (c flush) marshal(b []byte) { o.PutUint16(b, c.connectionHandle) }

type flushRP struct{ status uint8 }

// Write Page Timeout (0x0018)
type writePageTimeout struct{ pageTimeout uint16 }

func (c writePageTimeout) opcode() opcode   { return opWritePageTimeout }
func (c writePageTimeout) len() int         { return 2 }
func (c writePageTimeout) marshal(b []byte) { o.PutUint16(b, c.pageTimeout) }

type writePageTimeoutRP struct{}

// Write Class of Device (0x0024)
type writeClassOfDevice struct{ classOfDevice [3]byte }

func (c writeClassOfDevice) opcode() opcode   { return opWriteClassOfDevice }
func (c writeClassOfDevice) len() int         { return 3 }
func (c writeClassOfDevice) marshal(b []byte) { copy(b, c.classOfDevice[:]) }

type WriteClassOfDevRP struct{ status uint8 }

// Write Host Buffer Size (0x0033)
type hostBufferSize struct {
	hostACLDataPacketLength            uint16
	hostSynchronousDataPacketLength    uint8
	hostTotalNumACLDataPackets         uint16
	hostTotalNumSynchronousDataPackets uint16
}

func (c hostBufferSize) opcode() opcode { return opHostBufferSize }
func (c hostBufferSize) len() int       { return 7 }
func (c hostBufferSize) marshal(b []byte) {
	o.PutUint16(b[0:], c.hostACLDataPacketLength)
	o.PutUint8(b[2:], c.hostSynchronousDataPacketLength)
	o.PutUint16(b[3:], c.hostTotalNumACLDataPackets)
	o.PutUint16(b[5:], c.hostTotalNumSynchronousDataPackets)
}

type hostBufferSizeRP struct{ status uint8 }

// Write Inquiry Scan Type (0x0043)
type writeInquiryScanType struct{ scanType uint8 }

func (c writeInquiryScanType) opcode() opcode   { return opWriteInquiryScanType }
func (c writeInquiryScanType) len() int         { return 1 }
func (c writeInquiryScanType) marshal(b []byte) { b[0] = c.scanType }

type writeInquiryScanTypeRP struct{ status uint8 }

// Write Inquiry Mode (0x0045)
type writeInquiryMode struct {
	inquiryMode uint8
}

func (c writeInquiryMode) opcode() opcode   { return opWriteInquiryMode }
func (c writeInquiryMode) len() int         { return 1 }
func (c writeInquiryMode) marshal(b []byte) { b[0] = c.inquiryMode }

type writeInquiryModeRP struct{ status uint8 }

// Write Page Scan Type (0x0046)
type writePageScanType struct{ pageScanType uint8 }

func (c writePageScanType) opcode() opcode   { return opWritePageScanType }
func (c writePageScanType) len() int         { return 1 }
func (c writePageScanType) marshal(b []byte) { b[0] = c.pageScanType }

type writePageScanTypeRP struct{ status uint8 }

// Write Simple Pairing Mode (0x0056)
type writeSimplePairingMode struct{ simplePairingMode uint8 }

func (c writeSimplePairingMode) opcode() opcode   { return opWriteSimplePairingMode }
func (c writeSimplePairingMode) len() int         { return 1 }
func (c writeSimplePairingMode) marshal(b []byte) { b[0] = c.simplePairingMode }

type WriteSimplePairingModeRP struct{}

// Set Event Mask Page 2 (0x0063)
type setEventMaskPage2 struct{ eventMaskPage2 uint64 }

func (c setEventMaskPage2) opcode() opcode   { return opSetEventMaskPage2 }
func (c setEventMaskPage2) len() int         { return 8 }
func (c setEventMaskPage2) marshal(b []byte) { o.PutUint64(b, c.eventMaskPage2) }

type setEventMaskPage2RP struct{ status uint8 }

// Write LE Host Supported (0x006D)
type writeLEHostSupported struct {
	leSupportedHost    uint8
	simultaneousLEHost uint8
}

func (c writeLEHostSupported) opcode() opcode   { return opWriteLEHostSupported }
func (c writeLEHostSupported) len() int         { return 2 }
func (c writeLEHostSupported) marshal(b []byte) { b[0], b[1] = c.leSupportedHost, c.simultaneousLEHost }

type writeLeHostSupportedRP struct{ status uint8 }

// LE Controller Commands

// LE Set Event Mask (0x0001)
type leSetEventMask struct{ leEventMask uint64 }

func (c leSetEventMask) opcode() opcode   { return opLESetEventMask }
func (c leSetEventMask) len() int         { return 8 }
func (c leSetEventMask) marshal(b []byte) { o.PutUint64(b, c.leEventMask) }

type leSetEventMaskRP struct{ status uint8 }

// LE Read Buffer Size (0x0002)
type leReadBufferSize struct{}

func (c leReadBufferSize) opcode() opcode   { return opLEReadBufferSize }
func (c leReadBufferSize) len() int         { return 1 }
func (c leReadBufferSize) marshal(b []byte) {}

type leReadBufferSizeRP struct {
	status                     uint8
	hcLEACLDataPacketLength    uint16
	hcTotalNumLEACLDataPackets uint8
}

// LE Read Local Supported Features (0x0003)
type leReadLocalSupportedFeatures struct{}

func (c leReadLocalSupportedFeatures) opcode() opcode   { return opLEReadLocalSupportedFeatures }
func (c leReadLocalSupportedFeatures) len() int         { return 0 }
func (c leReadLocalSupportedFeatures) marshal(b []byte) {}

type leReadLocalSupportedFeaturesRP struct {
	status     uint8
	leFeatures uint64
}

// LE Set Random Address (0x0005)
type leSetRandomAddress struct{ randomAddress [6]byte }

func (c leSetRandomAddress) opcode() opcode   { return opLESetRandomAddress }
func (c leSetRandomAddress) len() int         { return 6 }
func (c leSetRandomAddress) marshal(b []byte) { o.PutMAC(b, c.randomAddress) }

type lESetRandomAddressRP struct{ status uint8 }

// LE Set Advertising Parameters (0x0006)
type leSetAdvertisingParameters struct {
	advertisingIntervalMin  uint16
	advertisingIntervalMax  uint16
	advertisingType         uint8
	ownAddressType          uint8
	directAddressType       uint8
	directAddress           [6]byte
	advertisingChannelMap   uint8
	advertisingFilterPolicy uint8
}

func (c leSetAdvertisingParameters) opcode() opcode { return opLESetAdvertisingParameters }
func (c leSetAdvertisingParameters) len() int       { return 15 }
func (c leSetAdvertisingParameters) marshal(b []byte) {
	o.PutUint16(b[0:], c.advertisingIntervalMin)
	o.PutUint16(b[2:], c.advertisingIntervalMax)
	o.PutUint8(b[4:], c.advertisingType)
	o.PutUint8(b[5:], c.ownAddressType)
	o.PutUint8(b[6:], c.directAddressType)
	o.PutMAC(b[7:], c.directAddress)
	o.PutUint8(b[13:], c.advertisingChannelMap)
	o.PutUint8(b[14:], c.advertisingFilterPolicy)
}

type leSetAdvertisingParametersRP struct{ status uint8 }

// LE Read Advertising Channel Tx Power (0x0007)
type leReadAdvertisingChannelTxPower struct{}

func (c leReadAdvertisingChannelTxPower) opcode() opcode   { return opLEReadAdvertisingChannelTxPower }
func (c leReadAdvertisingChannelTxPower) len() int         { return 0 }
func (c leReadAdvertisingChannelTxPower) marshal(b []byte) {}

type leReadAdvertisingChannelTxPowerRP struct {
	status             uint8
	transmitPowerLevel uint8
}

// LE Set Advertising Data (0x0008)
type leSetAdvertisingData struct {
	advertisingDataLength uint8
	advertisingData       [31]byte
}

func (c leSetAdvertisingData) opcode() opcode { return opLESetAdvertisingData }
func (c leSetAdvertisingData) len() int       { return 32 }
func (c leSetAdvertisingData) marshal(b []byte) {
	b[0] = c.advertisingDataLength
	copy(b[1:], c.advertisingData[:c.advertisingDataLength])
}

type leSetAdvertisingDataRP struct{ status uint8 }

// LE Set Scan Response Data (0x0009)
type leSetScanResponseData struct {
	scanResponseDataLength uint8
	scanResponseData       [31]byte
}

func (c leSetScanResponseData) opcode() opcode { return opLESetScanResponseData }
func (c leSetScanResponseData) len() int       { return 32 }
func (c leSetScanResponseData) marshal(b []byte) {
	b[0] = c.scanResponseDataLength
	copy(b[1:], c.scanResponseData[:c.scanResponseDataLength])
}

type leSetScanResponseDataRP struct{ status uint8 }

// LE Set Advertising Enable (0x000A)
type leSetAdvertiseEnable struct{ advertisingEnable uint8 }

func (c leSetAdvertiseEnable) opcode() opcode   { return opLESetAdvertiseEnable }
func (c leSetAdvertiseEnable) len() int         { return 1 }
func (c leSetAdvertiseEnable) marshal(b []byte) { b[0] = c.advertisingEnable }

type leSetAdvertiseEnableRP struct{ status uint8 }

// LE Set Scan Parameters (0x000B)
type leSetScanParameters struct {
	leScanType           uint8
	leScanInterval       uint16
	leScanWindow         uint16
	ownAddressType       uint8
	scanningFilterPolicy uint8
}

func (c leSetScanParameters) opcode() opcode { return opLESetScanParameters }
func (c leSetScanParameters) len() int       { return 7 }
func (c leSetScanParameters) marshal(b []byte) {
	o.PutUint8(b[0:], c.leScanType)
	o.PutUint16(b[1:], c.leScanInterval)
	o.PutUint16(b[3:], c.leScanWindow)
	o.PutUint8(b[5:], c.ownAddressType)
	o.PutUint8(b[6:], c.scanningFilterPolicy)
}

type leSetScanParametersRP struct{ status uint8 }

// LE Set Scan Enable (0x000C)
type leSetScanEnable struct {
	leScanEnable     uint8
	filterDuplicates uint8
}

func (c leSetScanEnable) opcode() opcode   { return opLESetScanEnable }
func (c leSetScanEnable) len() int         { return 2 }
func (c leSetScanEnable) marshal(b []byte) { b[0], b[1] = c.leScanEnable, c.filterDuplicates }

type leSetScanEnableRP struct{ status uint8 }

// LE Create Connection (0x000D)
type leCreateConn struct {
	leScanInterval        uint16
	leScanWindow          uint16
	initiatorFilterPolicy uint8
	peerAddressType       uint8
	peerAddress           [6]byte
	ownAddressType        uint8
	connIntervalMin       uint16
	connIntervalMax       uint16
	connLatency           uint16
	supervisionTimeout    uint16
	minimumCELength       uint16
	maximumCELength       uint16
}

func (c leCreateConn) opcode() opcode { return opLECreateConn }
func (c leCreateConn) len() int       { return 25 }
func (c leCreateConn) marshal(b []byte) {
	o.PutUint16(b[0:], c.leScanInterval)
	o.PutUint16(b[2:], c.leScanWindow)
	o.PutUint8(b[4:], c.initiatorFilterPolicy)
	o.PutUint8(b[5:], c.peerAddressType)
	o.PutMAC(b[6:], c.peerAddress)
	o.PutUint8(b[12:], c.ownAddressType)
	o.PutUint16(b[13:], c.connIntervalMin)
	o.PutUint16(b[15:], c.connIntervalMax)
	o.PutUint16(b[17:], c.connLatency)
	o.PutUint16(b[19:], c.supervisionTimeout)
	o.PutUint16(b[21:], c.minimumCELength)
	o.PutUint16(b[23:], c.maximumCELength)
}

type leCreateConnRP struct{}

// LE Create Connection Cancel (0x000E)
type leCreateConnCancel struct{}

func (c leCreateConnCancel) opcode() opcode   { return opLECreateConnCancel }
func (c leCreateConnCancel) len() int         { return 0 }
func (c leCreateConnCancel) marshal(b []byte) {}

type leCreateConnCancelRP struct{ status uint8 }

// LE Read White List Size (0x000F)
type leReadWhiteListSize struct{}

func (c leReadWhiteListSize) opcode() opcode   { return opLEReadWhiteListSize }
func (c leReadWhiteListSize) len() int         { return 0 }
func (c leReadWhiteListSize) marshal(b []byte) {}

type leReadWhiteListSizeRP struct {
	status        uint8
	whiteListSize uint8
}

// LE Clear White List (0x0010)
type leClearWhiteList struct{}

func (c leClearWhiteList) opcode() opcode   { return opLEClearWhiteList }
func (c leClearWhiteList) len() int         { return 0 }
func (c leClearWhiteList) marshal(b []byte) {}

type leClearWhiteListRP struct{ status uint8 }

// LE Add Device To White List (0x0011)
type leAddDeviceToWhiteList struct {
	addressType uint8
	address     [6]byte
}

func (c leAddDeviceToWhiteList) opcode() opcode { return opLEAddDeviceToWhiteList }
func (c leAddDeviceToWhiteList) len() int       { return 7 }
func (c leAddDeviceToWhiteList) marshal(b []byte) {
	b[0] = c.addressType
	o.PutMAC(b[1:], c.address)
}

type leAddDeviceToWhiteListRP struct{ status uint8 }

// LE Remove Device From White List (0x0012)
type leRemoveDeviceFromWhiteList struct {
	addressType uint8
	address     [6]byte
}

func (c leRemoveDeviceFromWhiteList) opcode() opcode { return opLERemoveDeviceFromWhiteList }
func (c leRemoveDeviceFromWhiteList) len() int       { return 7 }
func (c leRemoveDeviceFromWhiteList) marshal(b []byte) {
	b[0] = c.addressType
	o.PutMAC(b[1:], c.address)
}

type leRemoveDeviceFromWhiteListRP struct{ status uint8 }

// LE Connection Update (0x0013)
type leConnUpdate struct {
	connectionHandle   uint16
	connIntervalMin    uint16
	connIntervalMax    uint16
	connLatency        uint16
	supervisionTimeout uint16
	minimumCELength    uint16
	maximumCELength    uint16
}

func (c leConnUpdate) opcode() opcode { return opLEConnUpdate }
func (c leConnUpdate) len() int       { return 14 }
func (c leConnUpdate) marshal(b []byte) {
	o.PutUint16(b[0:], c.connectionHandle)
	o.PutUint16(b[2:], c.connIntervalMin)
	o.PutUint16(b[4:], c.connIntervalMax)
	o.PutUint16(b[6:], c.connLatency)
	o.PutUint16(b[8:], c.supervisionTimeout)
	o.PutUint16(b[10:], c.minimumCELength)
	o.PutUint16(b[12:], c.maximumCELength)
}

type leConnUpdateRP struct{}

// LE Set Host Channel Classification (0x0014)
type leSetHostChannelClassification struct{ channelMap [5]byte }

func (c leSetHostChannelClassification) opcode() opcode   { return opLESetHostChannelClassification }
func (c leSetHostChannelClassification) len() int         { return 5 }
func (c leSetHostChannelClassification) marshal(b []byte) { copy(b, c.channelMap[:]) }

type leSetHostChannelClassificationRP struct{ status uint8 }

// LE Read Channel Map (0x0015)
type leReadChannelMap struct{ connectionHandle uint16 }

func (c leReadChannelMap) opcode() opcode   { return opLEReadChannelMap }
func (c leReadChannelMap) len() int         { return 2 }
func (c leReadChannelMap) marshal(b []byte) { o.PutUint16(b, c.connectionHandle) }

type leReadChannelMapRP struct {
	status           uint8
	connectionHandle uint16
	channelMap       [5]byte
}

// LE Read Remote Used Features (0x0016)
type leReadRemoteUsedFeatures struct{ connectionHandle uint16 }

func (c leReadRemoteUsedFeatures) opcode() opcode   { return opLEReadRemoteUsedFeatures }
func (c leReadRemoteUsedFeatures) len() int         { return 8 }
func (c leReadRemoteUsedFeatures) marshal(b []byte) { o.PutUint16(b, c.connectionHandle) }

type leReadRemoteUsedFeaturesRP struct{}

// LE Encrypt (0x0017)
type leEncrypt struct {
	key           [16]byte
	plaintextData [16]byte
}

func (c leEncrypt) opcode() opcode { return opLEEncrypt }
func (c leEncrypt) len() int       { return 32 }
func (c leEncrypt) marshal(b []byte) {
	copy(b[0:], c.key[:])
	copy(b[16:], c.plaintextData[:])
}

type leEncryptRP struct {
	stauts        uint8
	encryptedData [16]byte
}

// LE Rand (0x0018)
type leRand struct{}

func (c leRand) opcode() opcode   { return opLERand }
func (c leRand) len() int         { return 0 }
func (c leRand) marshal(b []byte) {}

type leRandRP struct {
	status       uint8
	randomNumber uint64
}

// LE Start Encryption (0x0019)
type leStartEncryption struct {
	connectionHandle     uint16
	randomNumber         uint64
	encryptedDiversifier uint16
	longTermKey          [16]byte
}

func (c leStartEncryption) opcode() opcode { return opLEStartEncryption }
func (c leStartEncryption) len() int       { return 28 }
func (c leStartEncryption) marshal(b []byte) {
	o.PutUint16(b[0:], c.connectionHandle)
	o.PutUint64(b[2:], c.randomNumber)
	o.PutUint16(b[10:], c.encryptedDiversifier)
	copy(b[12:], c.longTermKey[:])
}

type leStartEncryptionRP struct{}

// LE Long Term Key Reply (0x001A)
type leLTKReply struct {
	connectionHandle uint16
	longTermKey      [16]byte
}

func (c leLTKReply) opcode() opcode { return opLELTKReply }
func (c leLTKReply) len() int       { return 18 }
func (c leLTKReply) marshal(b []byte) {
	o.PutUint16(b[0:], c.connectionHandle)
	copy(b[2:], c.longTermKey[:])
}

type leLTKReplyRP struct {
	status           uint8
	connectionHandle uint16
}

// LE Long Term Key  Negative Reply (0x001B)
type leLTKNegReply struct{ connectionHandle uint16 }

func (c leLTKNegReply) opcode() opcode   { return opLELTKNegReply }
func (c leLTKNegReply) len() int         { return 2 }
func (c leLTKNegReply) marshal(b []byte) { o.PutUint16(b, c.connectionHandle) }

type leLTKNegReplyRP struct {
	status           uint8
	connectionHandle uint16
}

// LE Read Supported States (0x001C)
type leReadSupportedStates struct{}

func (c leReadSupportedStates) opcode() opcode   { return opLEReadSupportedStates }
func (c leReadSupportedStates) len() int         { return 0 }
func (c leReadSupportedStates) marshal(b []byte) {}

type leReadSupportedStatesRP struct {
	status   uint8
	leStates [8]byte
}

// LE Reciever Test (0x001D)
type leReceiverTest struct{ rxChannel uint8 }

func (c leReceiverTest) opcode() opcode   { return opLEReceiverTest }
func (c leReceiverTest) len() int         { return 1 }
func (c leReceiverTest) marshal(b []byte) { b[0] = c.rxChannel }

type leReceiverTestRP struct{ status uint8 }

// LE Transmitter Test (0x001E)
type leTransmitterTest struct {
	txChannel        uint8
	lengthOfTestData uint8
	packetPayload    uint8
}

func (c leTransmitterTest) opcode() opcode { return opLETransmitterTest }
func (c leTransmitterTest) len() int       { return 3 }
func (c leTransmitterTest) marshal(b []byte) {
	b[0], b[1], b[2] = c.txChannel, c.lengthOfTestData, c.packetPayload
}

type leTransmitterTestRP struct{ status uint8 }

// LE Test End (0x001F)
type leTestEnd struct{}

func (c leTestEnd) opcode() opcode   { return opLETestEnd }
func (c leTestEnd) len() int         { return 0 }
func (c leTestEnd) marshal(b []byte) {}

type leTestEndRP struct {
	status          uint8
	numberOfPackets uint16
}

// LE Remote Connection Parameters Reply (0x0020)
type leRemoteConnectionParameterReply struct {
	connectionHandle uint16
	intervalMin      uint16
	intervalMax      uint16
	latency          uint16
	timeout          uint16
	minimumCELength  uint16
	maximumCELength  uint16
}

func (c leRemoteConnectionParameterReply) opcode() opcode { return opLERemoteConnectionParameterReply }
func (c leRemoteConnectionParameterReply) len() int       { return 14 }
func (c leRemoteConnectionParameterReply) marshal(b []byte) {
	o.PutUint16(b[0:], c.connectionHandle)
	o.PutUint16(b[2:], c.intervalMin)
	o.PutUint16(b[4:], c.intervalMax)
	o.PutUint16(b[6:], c.latency)
	o.PutUint16(b[8:], c.timeout)
	o.PutUint16(b[10:], c.minimumCELength)
	o.PutUint16(b[12:], c.maximumCELength)
}

type leRemoteConnectionParameterReplyRP struct {
	status           uint8
	connectionHandle uint16
}

// LE Remote Connection Parameters Negative Reply (0x0021)
type leRemoteConnectionParameterNegReply struct {
	connectionHandle uint16
	reason           uint8
}

func (c leRemoteConnectionParameterNegReply) opcode() opcode {
	return opLERemoteConnectionParameterNegReply
}
func (c leRemoteConnectionParameterNegReply) len() int { return 3 }
func (c leRemoteConnectionParameterNegReply) marshal(b []byte) {
	o.PutUint16(b[0:], c.connectionHandle)
	b[2] = c.reason
}

type leRemoteConnectionParameterNegReplyRP struct {
	status           uint8
	connectionHandle uint16
}
