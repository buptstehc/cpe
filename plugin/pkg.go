package plugin

import (
	"encoding/json"
	"encoding/binary"
)

type Packet struct {
	RPCMethod string
	ID uint32
	Result int8
	PONG string
	Interval string
}

type BootInitiationReqPacket struct {
	RPCMethod string
	ID uint32
	MAC string
	PROTVersion string
}

type BootInitiationRespPacket struct {
	ID uint32
	Result int8
	ChallengeCode string
}

type RegisterReqPacket struct {
	RPCMethod string
	ID uint32
	MAC string
	CheckGateway string
	DevRND string
}

type RegisterRespPacket struct {
	ID uint32
	Result int8
	CheckPlatform string
}

type HBPacket struct {
	RPCMethod string
	PING      string
}

func addHeader(body []byte) []byte {
	l := len(body)
	msg := make([]byte, l + 4)
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

func (self *BootInitiationReqPacket) Serialize() ([]byte, bool){
	body, err := json.Marshal(self)
	if err != nil {
		return nil, false
	}

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

func (self *RegisterReqPacket) Serialize() ([]byte, bool){
	body, err := json.Marshal(self)
	if err != nil {
		return nil, false
	}

	return addHeader(body), true
}

func NewHBPacket() (*HBPacket) {
	b := &HBPacket{}
	b.RPCMethod = "Hb"
	b.PING = "PING"

	return b
}

func (self *HBPacket) Serialize() ([]byte, bool){
	body, err := json.Marshal(self)
	if err != nil {
		return nil, false
	}

	return addHeader(body), true
}
