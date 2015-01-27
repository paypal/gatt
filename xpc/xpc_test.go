package xpc

import (
	"testing"
)

func CheckUUID(t *testing.T, v interface{}) UUID {
	if uuid, ok := v.(UUID); ok {
		return uuid
	} else {
		t.Errorf("not a UUID: %#v\n", v)
		return uuid
	}
}

func TestConvertUUID(t *testing.T) {
	uuid := MakeUUID("00112233445566778899aabbccddeeff")

	xv := goToXpc(uuid)
	v := xpcToGo(xv)

	xpc_release(xv)

	uuid2 := CheckUUID(t, v)

	if uuid != uuid2 {
		t.Errorf("expected %#v got %#v\n", uuid, uuid2)
	}
}

func TestConvertSlice(t *testing.T) {
	arr := []string{"one", "two", "three"}

	xv := goToXpc(arr)
	v := xpcToGo(xv)

	xpc_release(xv)

	if arr2, ok := v.(array); !ok {
		t.Errorf("not an array: %#v\n", v)
	} else if len(arr) != len(arr2) {
		t.Errorf("expected %#v got %#v\n", arr, arr2)
	} else {
		for i := range arr {
			if arr[i] != arr2[i] {
				t.Errorf("expected array[%d]: %#v got %#v\n", i, arr[i], arr2[i])
			}
		}
	}
}

func TestConvertSliceUUID(t *testing.T) {
	arr := []UUID{MakeUUID("0000000000000000"), MakeUUID("1111111111111111"), MakeUUID("2222222222222222")}

	xv := goToXpc(arr)
	v := xpcToGo(xv)

	xpc_release(xv)

	if arr2, ok := v.(array); !ok {
		t.Errorf("not an array: %#v\n", v)
	} else if len(arr) != len(arr2) {
		t.Errorf("expected %#v got %#v\n", arr, arr2)
	} else {
		for i := range arr {
			uuid1 := CheckUUID(t, arr[i])
			uuid2 := CheckUUID(t, arr2[i])

			if uuid1 != uuid2 {
				t.Errorf("expected array[%d]: %#v got %#v\n", i, arr[i], arr2[i])
			}
		}
	}
}

func TestConvertMap(t *testing.T) {
	d := dict{
		"number": int64(42),
		"text":   "hello gopher",
		"uuid":   MakeUUID("aabbccddeeff00112233445566778899"),
	}

	xv := goToXpc(d)
	v := xpcToGo(xv)

	xpc_release(xv)

	if d2, ok := v.(dict); !ok {
		t.Errorf("not a map: %#v", v)
	} else if len(d) != len(d2) {
		t.Errorf("expected %#v got %#v\n", d, d2)
	} else {
		fail := false

		for k, v := range d {
			if v != d2[k] {
				t.Logf("expected map[%s]: %#v got %#v\n", k, v, d2[k])
				fail = true
			}
		}

		if fail {
			t.Error("test failed")
		}
	}
}
