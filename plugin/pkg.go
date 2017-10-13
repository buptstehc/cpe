package plugin

import (
	"encoding/binary"
	"github.com/pquerna/ffjson/ffjson"
)

type Packet struct {
	RPCMethod string
}

type RespPacket struct {
	ID     uint32
	Result int8
}

type InstallPacket struct {
	ID           uint32
	Plugin_Name  string
	Version      string
	Download_url string
	Plugin_size  string
	Run          bool
}

type InstalledPacket struct {
	Plugin_Name  string
	Version      string
	Run          uint8
}

type InstallQueryPacket struct {
	ID uint32
}

type InstallQueryRespPacket struct {
	ID      uint32
	Result  int8
	Percent string
}

type InstallCancelPacket struct {
	ID          uint32
	Plugin_Name string
}

type InstallCancelRespPacket struct {
	ID     uint32
	Result int8
}

type UnInstallPacket struct {
	ID          uint32
	Plugin_Name string
}

type UnInstallRespPacket struct {
	ID     uint32
	Result int8
}

type StopPacket struct {
	ID          uint32
	Plugin_Name string
}

type StopRespPacket struct {
	ID     uint32
	Result int8
}

type RunPacket struct {
	ID          uint32
	Plugin_Name string
}

type RunRespPacket struct {
	ID     uint32
	Result int8
}

type FactoryPluginPacket struct {
	ID          uint32
	Plugin_Name string
}

type FactoryPluginRespPacket struct {
	ID     uint32
	Result int8
}

type ListPluginPacket struct {
	ID uint32
}

type ListPluginRespPacket struct {
	ID     uint32
	Result int8
	Plugin []InstalledPacket
}

type BootInitiationReqPacket struct {
	RPCMethod   string
	ID          uint32
	MAC         string
	PROTVersion string
}

type BootInitiationRespPacket struct {
	ID            uint32
	Result        int8
	ChallengeCode string
}

type RegisterReqPacket struct {
	RPCMethod    string
	ID           uint32
	MAC          string
	CheckGateway string
	DevRND       string
}

type RegisterRespPacket struct {
	ID            uint32
	Result        int8
	CheckPlatform string
}

type HBPacket struct {
	RPCMethod string
	PING      string
}

func addHeader(body []byte) []byte {
	l := len(body)
	msg := make([]byte, l+4)
	binary.BigEndian.PutUint32(msg, uint32(l))
	copy(msg[4:], body)

	return msg
}

func NewBootInitiationPacket(id uint32, mac string) (*BootInitiationReqPacket) {
	b := &BootInitiationReqPacket{}
	b.RPCMethod = "BootInitiation"
	b.PROTVersion = "1.0"
	b.ID = id
	b.MAC = mac

	return b
}

func (self *BootInitiationReqPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func NewRegisterPacket(id uint32, mac, checkGateway, devRnd string) (*RegisterReqPacket) {
	b := &RegisterReqPacket{}
	b.ID = id
	b.MAC = mac
	b.CheckGateway = checkGateway
	b.DevRND = devRnd
	b.RPCMethod = "Register"

	return b
}

func (self *RegisterReqPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func NewHBPacket() (*HBPacket) {
	b := &HBPacket{}
	b.RPCMethod = "Hb"
	b.PING = "PING"

	return b
}

func (self *RespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *HBPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *InstallQueryRespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *InstallCancelRespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *UnInstallRespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *StopRespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *RunRespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *FactoryPluginRespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *ListPluginRespPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}

func (self *InstalledPacket) Serialize() ([]byte, bool) {
	body, err := ffjson.Marshal(self)
	if err != nil {
		return nil, false
	}
	ffjson.Pool(body)

	return addHeader(body), true
}