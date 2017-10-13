package test

import (
	"testing"
	"cpe/plugin"
	"fmt"
	"github.com/pquerna/ffjson/ffjson"
)

func Test_JSON(t *testing.T) {
	msg := []byte(`{"RPCMethod":"Install"}`)
	pkt := plugin.Packet{}
	ffjson.Unmarshal(msg, &pkt)

	fmt.Printf("%+v\n", pkt)
}
