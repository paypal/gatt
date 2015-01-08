package gatt

type Descriptor struct {
	uuid  UUID
	char  Characteristic
	value []byte // static value
}

func (d Descriptor) UUID() UUID { return d.uuid }

// FIXME: return pointer or copy
func (d Descriptor) Characteristic() Characteristic { return d.char }
func (d Descriptor) Value() []byte                  { return d.value }
