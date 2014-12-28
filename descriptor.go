package gatt

type desc struct {
	uuid  UUID
	value []byte // static value
}

func (d *desc) UUID() UUID {
	return d.uuid
}
