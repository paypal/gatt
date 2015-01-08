package gatt

type Peripheral struct {
	Name          string
	Services      []Service
	State         string
	Advertisement Advertisement

	// Notified is called when subscribed characteristic is notified.
	Notified func(*Peripheral, *Characteristic, error)

	// NameChanged is called whenever the peripheral GAP device name has changed.
	NameChanged func(*Peripheral)

	// ServicedModified is called when one or more service of a peripheral have changed.
	// A list of invalid service is provided in the parameter.
	ServicesModified func(*Peripheral, []*Service)
}
