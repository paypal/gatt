package gatt

type desc struct {
	uuid  UUID
	value []byte // static value
}

func (d *desc) handle(n uint16) handle {
	return handle{
		typ:    typDescriptor,
		n:      n,
		uuid:   d.uuid,
		attr:   d,
		props:  charRead,
		secure: 0,
		value:  d.value,
	}
}

func (d *desc) UUID() UUID {
	return d.uuid
}
