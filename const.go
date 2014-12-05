package gatt

// This file includes constants from the BLE spec.

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
