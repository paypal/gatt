package gatt

// A Service is a BLE service.
// Calls to AddCharacteristic must occur before the
// service is used by a server.
type Service struct {
	uuid  UUID
	chars []*Characteristic
}

// AddCharacteristic adds a characteristic to a service.
// AddCharacteristic panics if the service already contains
// another characteristic with the same UUID.
func (s *Service) AddCharacteristic(u UUID) *Characteristic {
	// TODO: write test for this panic
	for _, char := range s.chars {
		if uuidEqual(char.uuid, u) {
			panic("service already contains a characteristic with uuid " + u.String())
		}
	}

	char := &Characteristic{
		service: s,
		uuid:    u,
	}
	s.chars = append(s.chars, char)
	return char
}

func (s *Service) generateHandles(n uint16) (uint16, []handle) {
	h := handle{
		typ:    typService,
		n:      n,
		uuid:   s.uuid,
		attr:   s,
		startn: n,
		// endn set later
	}
	handles := []handle{h}

	for _, char := range s.chars {
		n++
		var hh []handle
		n, hh = char.generateHandles(n)
		handles = append(handles, hh...)
	}

	handles[0].endn = n
	n++
	return n, handles
}

// UUID returns the service's UUID.
func (s *Service) UUID() UUID {
	return s.uuid
}
