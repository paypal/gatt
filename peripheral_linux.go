package gatt

// DiscoverServices discover the specified services of the peripheral.
// If the specified services is set to nil, all the available services of the peripheral are returned.
func (p *Peripheral) DiscoverServices(s []UUID) ([]*Service, error) { return nil, nil }

// DiscoverIncludedServices discovers the specified included services of a service.
// If the specified services is set to nil, all the available services of the peripheral are returned.
func (p *Peripheral) DiscoverIncludedServices(s []UUID, svc UUID) ([]*Service, error) { return nil, nil }

// DiscoverCharacteristics discovers the specified characteristics of a service.
func (p *Peripheral) DiscoverCharacteristics(c []UUID, svc UUID) ([]*Characteristic, error) {
	return nil, nil
}

// DiscoverDescriptors discovers the descriptors of a characteristic.
func (p *Peripheral) DiscoverDescriptors(d []UUID, svc UUID) ([]*Descriptor, error) { return nil, nil }

// ReadCharacteristic retrieves the value of a specified characteristic.
func (p *Peripheral) ReadCharacteristic(c *Characteristic) ([]byte, error) { return nil, nil }

// ReadDescriptor retrieves the value of a specified characteristic descriptor.
func (p *Peripheral) ReadDescriptor(d *Descriptor) ([]byte, error) { return nil, nil }

// WriteCharacteristic writes the value of a characteristic.
func (p *Peripheral) WriteCharacteristic(c *Characteristic, b []byte, resp bool) error { return nil }

// WriteDescriptor writes the value of a characteristic descriptor.
func (p *Peripheral) WriteDescriptor(d *Descriptor, b []byte) error { return nil }

// SetNotifyValue sets notifications or indications for the value of a specified characteristic.
func (p *Peripheral) SetNotifyValue(c *Characteristic, b bool) error { return nil }

// ReadRSSI retrieves the current RSSI value for the peripheral while it is connected to the central manager.
func (p *Peripheral) ReadRSSI() int { return -1 }
