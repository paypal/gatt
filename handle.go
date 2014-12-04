package gatt

type handleType int

const (
	typService handleType = iota
	typCharacteristic
	typDescriptor
	typCharacteristicValue
	typIncludedService
)

// handle is a BLE handle. It is not exported;
// managing handles is an implementation detail.
// TODO: The organization of this is borrowed
// straight from bleno, as the union of all
// the types involved. It could be made much
// tighter and more typesafe with a bit of effort,
// once some l2cap unit tests are in place.
type handle struct {
	n      uint16 // gatt handle number
	startn uint16
	valuen uint16
	endn   uint16
	typ    handleType
	uuid   UUID
	attr   interface{}
	props  uint
	secure uint
	value  []byte
}

// isPrimaryService reports whether this handle is
// the primary service with uuid uuid.
func (h handle) isPrimaryService(uuid UUID) bool {
	return h.typ == typService && uuidEqual(uuid, h.uuid)
}

// isCharacteristic reports whether this handle is the
// characteristic with uuid uuid.
func (h handle) isCharacteristic(uuid UUID) bool {
	return h.typ == typCharacteristic && uuidEqual(uuid, h.uuid)
}

// isDescriptor reports whether this handle is the
// descriptor with uuid uuid.
func (h handle) isDescriptor(uuid UUID) bool {
	return h.typ == typDescriptor && uuidEqual(uuid, h.uuid)
}

func generateHandles(name string, svcs []*Service, base uint16) *handleRange {
	svcs = append(defaultServices(name), svcs...)
	var handles []handle
	n := base

	last := len(svcs) - 1
	for i, svc := range svcs {
		var hh []handle
		n, hh = svc.generateHandles(n, i == last)
		handles = append(handles, hh...)
	}

	return &handleRange{hh: handles, base: base}
}

func defaultServices(name string) []*Service {
	gapService := &Service{
		uuid: gatAttrGAPUUID,
		chars: []*Characteristic{
			&Characteristic{
				uuid:   gattAttrDeviceNameUUID,
				props:  charRead,
				secure: charRead,
				value:  []byte(name),
			},
			&Characteristic{
				uuid:   gattAttrAppearanceUUID,
				props:  charRead,
				secure: charRead,
				value:  gapCharAppearanceGenericComputer,
			},
		},
	}

	gattService := &Service{uuid: gatAttrGATTUUID}
	return []*Service{gapService, gattService}
}

// A handleRange is a contiguous range of handles.
type handleRange struct {
	hh   []handle
	base uint16 // handle number for first handle in hh
}

const (
	tooSmall = -1
	tooLarge = -2
)

// idx returns the index into hh corresponding to handle n.
// If n is too small, idx returns tooSmall (-1).
// If n is too large, idx returns tooLarge (-2).
func (r *handleRange) idx(n int) int {
	if n < int(r.base) {
		return tooSmall
	}
	if int(n) >= int(r.base)+len(r.hh) {
		return tooLarge
	}
	return n - int(r.base)
}

// At returns handle n.
func (r *handleRange) At(n uint16) (h handle, ok bool) {
	i := r.idx(int(n))
	if i < 0 {
		return handle{}, false
	}
	return r.hh[i], true
}

// Subrange returns handles in range [start, end]; it may
// return an empty slice. Subrange does not panic for
// out-of-range start or end.
func (r *handleRange) Subrange(start, end uint16) []handle {
	startidx := r.idx(int(start))
	switch startidx {
	case tooSmall:
		startidx = 0
	case tooLarge:
		return []handle{}
	}

	endidx := r.idx(int(end) + 1) // [start, end] includes its upper bound!
	switch endidx {
	case tooSmall:
		return []handle{}
	case tooLarge:
		endidx = len(r.hh)
	}
	return r.hh[startidx:endidx]
}
