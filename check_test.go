package gobearmon

import "testing"

func TestPing(t *testing.T) {
	if checkFuncs == nil {
		checkInit()
	}
	data := "{\"target\":\"quebec.lunaghost.com\"}"
	err := checkFuncs["icmp"](data)
	if err != nil {
		t.Fatal(err)
	}
}
