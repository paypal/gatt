package gatt

// A Service is a BLE service.
// Calls to AddCharacteristic must occur before the
// service is used by a server.
type Service struct {
	uuid  UUID
	chars []*Characteristic
}

func NewService(u UUID) *Service { return &Service{uuid: u} }

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

	char := &Characteristic{service: s, uuid: u}
	s.chars = append(s.chars, char)
	return char
}

func (s *Service) Characteristics() []*Characteristic { return s.chars }

// FIXME:
func (s *Service) IsPrimary() bool { return true }

// UUID returns the service's UUID.
func (s *Service) UUID() UUID { return s.uuid }
