package legs

import (
	"encoding/json"
	"github.com/ipfs/go-cid"
	"testing"
)

func TestLoadChecklist(t *testing.T) {
	a := make(map[string]struct{})
	c, _ := cid.Decode("bafyreeekhqdl3lkqityjp22727y5s2ia")
	a[c.String()] = struct{}{}

	res, _ := json.Marshal(a)

	t.Log(string(res))

	r := make(map[string]struct{})
	err := json.Unmarshal(res, &r)
	if err != nil {
		t.Fatal(err)
	}

}
