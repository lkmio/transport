package transport

import (
	"context"
	"net"
)

type UDPServer struct {
	ReuseServer
	udp []net.PacketConn
}

func (u *UDPServer) Bind(addr net.Addr) error {
	u.ctx, u.cancel = context.WithCancel(context.Background())

	number := u.ReuseServer.ComputeConcurrentNumber()
	for i := 0; i < number; i++ {
		lc := net.ListenConfig{
			Control: u.ReuseServer.GetSetOptFunc(),
		}

		socket, err := lc.ListenPacket(u.ctx, "udp", addr.String())
		if err != nil {
			u.Close()
			return err
		}

		u.udp = append(u.udp, socket)
	}

	u.setListenAddr(addr)
	return nil
}

func (u *UDPServer) Receive() {
	Assert(u.handler != nil)
	Assert(len(u.udp) > 0)

	for _, conn := range u.udp {
		go readDataFromUDPConn(u.ctx, conn, u.handler)
	}
}

func (u *UDPServer) Close() {
	for _, conn := range u.udp {
		conn.Close()
	}

	u.transport.Close()
}

func readDataFromUDPConn(ctx context.Context, conn net.PacketConn, handler Handler) {
	// 音视频UDP收流都使用jitter buffer处理, 难免还是会拷贝一次, 所以读取数据时不使用外部缓冲区.
	bytes := make([]byte, 1500)

	for ctx.Err() == nil {
		n, addr, err := conn.ReadFrom(bytes)
		if err != nil {
			println(err.Error())
			if n == 0 {
				break
			}
		}

		if n > 0 && handler != nil {
			c := &Conn{conn: &UDPConn{conn, conn.LocalAddr(), addr}, closeCb: handler.OnDisConnected}
			handler.OnPacket(c, bytes[:n])
		}
	}
}
