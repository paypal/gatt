package gatt

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

func attErrorResp(op byte, h uint16, s uint8) []byte {
	return attErr{opcode: op, handle: h, status: s}.Marshal()
}

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

type attErr struct {
	opcode uint8
	handle uint16
	status uint8
}

// TODO: Reformulate in a way that lets the caller avoid allocs.
// Accept a []byte? Write directly to an io.Writer?
func (e attErr) Marshal() []byte {
	// little-endian encoding for handle
	return []byte{attOpError, e.opcode, byte(e.handle), byte(e.handle >> 8), e.status}
}
