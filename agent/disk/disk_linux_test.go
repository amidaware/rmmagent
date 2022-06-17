package disk

import (
	"testing"
)

func TestGetDisks(t *testing.T) {
	disks := GetDisks()
	if len(disks) == 0 {
		t.Fatalf("Could not get disks on linux system.")
	}
	
	t.Logf("Got %d disks on linux system", len(disks))
}