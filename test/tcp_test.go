package test

import (
	"testing"
	"cpe/plugin"
)

func Test_doInstall(t *testing.T) {
	pkt := plugin.InstallPacket{ID:1, Plugin_Name:"foo", Version:"v1.0", Download_url:"http://x.com", Plugin_size:"100"}

}
