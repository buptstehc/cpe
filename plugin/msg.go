package plugin

const (
	E_ABORT      = 0x00
	E_BOOT_SENT  = 0x01
	E_BOOT_REPLY = 0x02
	E_REG_SENT   = 0x03
	E_REG_REPLY  = 0x04
	E_PING       = 0x05
	E_PONG       = 0x06
)

//用于各个网关向主上报事件
type Msg struct {
	ID    uint32
	Event uint8
}

type Metric struct {
	Aborts    uint64
	BootSent  uint64
	RegSent   uint64
	BootReply uint64
	RegReply  uint64
	Pings     uint64
	Pongs     uint64
}
