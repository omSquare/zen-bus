package zbus

import "testing"

// TestRegistration tests the process of registering and unregistering of slaves.
func TestRegistration(t *testing.T) {
	a := &arp{}
	if a.num != 0 {
		t.Fatalf("A brand new ARP contains slaves")
	}

	// register 1st slave
	dev1 := Device{Id: [8]byte{0x01, 0x03, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}}
	s1, err := a.register(&dev1)
	if err != nil || s1 == nil {
		t.Fatalf("Failed to register device")
	}

	if s1.id != dev1.Id {
		t.Errorf("Registered slave has invalid UDID")
	}

	if !s1.active() {
		t.Errorf("Registered slave is not active")
	}

	if a.num != 1 {
		t.Errorf("Invalid number of registered devices, 1 expected, got %v", a.num)
	}

	t.Logf("1st slave registered with address %02x", s1.addr)

	// register 2nd slave
	dev2 := Device{Id: [8]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x00, 0x00}}
	s2, err := a.register(&dev2)
	if err != nil || s2 == nil {
		t.Fatalf("Failed to register device")
	}

	if s2.id != dev2.Id {
		t.Errorf("Registered slave has invalid UDID")
	}

	if !s2.active() {
		t.Errorf("Registered slave is not active")
	}

	if a.num != 2 {
		t.Errorf("Invalid number of registered devices, 2 expected, got %v", a.num)
	}

	t.Logf("2nd slave registered with address %02x", s2.addr)

	// unregister 1st slave
	t.Logf("Unregistering 1st slave")
	a.unregister(s1)

	if a.num != 1 {
		t.Errorf("Invalid number of registered devices, 1 expected, got %v", a.num)
	}

	if a.slaves[s1.index()] != nil {
		t.Errorf("1st slave still registered")
	}

	if a.slaves[s2.index()] == nil {
		t.Errorf("2nd slave not registered")
	}
}
