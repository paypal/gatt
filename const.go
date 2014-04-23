package gatt

// This file includes constants from the BLE spec.

const (
	attOpError           = 0x01
	attOpMtuReq          = 0x02
	attOpMtuResp         = 0x03
	attOpFindInfoReq     = 0x04
	attOpFindInfoResp    = 0x05
	attOpFindByTypeReq   = 0x06
	attOpFindByTypeResp  = 0x07
	attOpReadByTypeReq   = 0x08
	attOpReadByTypeResp  = 0x09
	attOpReadReq         = 0x0a
	attOpReadResp        = 0x0b
	attOpReadBlobReq     = 0x0c
	attOpReadBlobResp    = 0x0d
	attOpReadMultiReq    = 0x0e
	attOpReadMultiResp   = 0x0f
	attOpReadByGroupReq  = 0x10
	attOpReadByGroupResp = 0x11
	attOpWriteReq        = 0x12
	attOpWriteResp       = 0x13
	attOpWriteCmd        = 0x52
	attOpPrepWriteReq    = 0x16
	attOpPrepWriteResp   = 0x17
	attOpExecWriteReq    = 0x18
	attOpExecWriteResp   = 0x19
	attOpHandleNotify    = 0x1b
	attOpHandleInd       = 0x1d
	attOpHandleCnf       = 0x1e
	attOpSignedWriteCmd  = 0xd2
)

const (
	attEcodeSuccess           = 0x00
	attEcodeInvalidHandle     = 0x01
	attEcodeReadNotPerm       = 0x02
	attEcodeWriteNotPerm      = 0x03
	attEcodeInvalidPDU        = 0x04
	attEcodeAuthentication    = 0x05
	attEcodeReqNotSupp        = 0x06
	attEcodeInvalidOffset     = 0x07
	attEcodeAuthorization     = 0x08
	attEcodePrepQueueFull     = 0x09
	attEcodeAttrNotFound      = 0x0a
	attEcodeAttrNotLong       = 0x0b
	attEcodeInsuffEncrKeySize = 0x0c
	attEcodeInvalAttrValueLen = 0x0d
	attEcodeUnlikely          = 0x0e
	attEcodeInsuffEnc         = 0x0f
	attEcodeUnsuppGrpType     = 0x10
	attEcodeInsuffResources   = 0x11
)

// attRespFor maps from att request
// codes to att response codes.
var attRespFor = map[byte]byte{
	attOpMtuReq:         attOpMtuResp,
	attOpFindInfoReq:    attOpFindInfoResp,
	attOpFindByTypeReq:  attOpFindByTypeResp,
	attOpReadByTypeReq:  attOpReadByTypeResp,
	attOpReadReq:        attOpReadResp,
	attOpReadBlobReq:    attOpReadBlobResp,
	attOpReadMultiReq:   attOpReadMultiResp,
	attOpReadByGroupReq: attOpReadByGroupResp,
	attOpWriteReq:       attOpWriteResp,
	attOpPrepWriteReq:   attOpPrepWriteResp,
	attOpExecWriteReq:   attOpExecWriteResp,
}

var (
	gatAttrGAPUUID  = UUID16(0x1800)
	gatAttrGATTUUID = UUID16(0x1801)

	gattAttrPrimaryServiceUUID   = UUID16(0x2800)
	gattAttrSecondaryServiceUUID = UUID16(0x2801)
	gattAttrIncludeUUID          = UUID16(0x2802)
	gattAttrCharacteristicUUID   = UUID16(0x2803)

	gattAttrClientCharacteristicConfigUUID = UUID16(0x2902)
	gattAttrServerCharacteristicConfigUUID = UUID16(0x2903)

	gattAttrDeviceNameUUID = UUID16(0x2A00)
	gattAttrAppearanceUUID = UUID16(0x2A01)
)

// https://developer.bluetooth.org/gatt/characteristics/Pages/CharacteristicViewer.aspx?u=org.bluetooth.characteristic.gap.appearance.xml
var gapCharAppearanceGenericComputer = []byte{0x00, 0x80}

const gattCCCNotifyFlag = 1
