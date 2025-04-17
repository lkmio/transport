package transport

import (
	"context"
	"net"
	"sync/atomic"
	"time"
)

// Conn 为连接句柄扩展public data字段
type Conn struct {
	conn   net.Conn
	Data   interface{} // 绑定参数
	closed atomic.Bool

	pendingSendQueue chan []byte // 等待发送的数据队列
	cancelFunc       func()      // 退出异步发送的协程
	cancelCtx        context.Context

	closeCb func(conn net.Conn, err error) // 主动调用Close时回调
}

func (c *Conn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

// EnableAsyncWriteMode 使用异步发送数据包
func (c *Conn) EnableAsyncWriteMode(queueSize int) {
	c.pendingSendQueue = make(chan []byte, queueSize)
	c.cancelCtx, c.cancelFunc = context.WithCancel(context.Background())
	go c.doAsyncWrite()
}

func (c *Conn) doAsyncWrite() {
	for {
		select {
		case <-c.cancelCtx.Done():
			return
		case data := <-c.pendingSendQueue:
			_, err := c.conn.Write(data)

			if err != nil {
				println(err.Error())

				// 发送失败, 并且已经关闭, 退出协程
				if c.closed.Load() {
					return
				}
			}
			break
		}
	}
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if c.cancelCtx != nil {
		select {
		case c.pendingSendQueue <- b:
			return len(b), nil
		default:
			return 0, &ZeroWindowSizeError{}
		}
	} else {
		return c.conn.Write(b)
	}
}

func (c *Conn) Close() error {
	if closed := c.closed.Swap(true); closed {
		return nil
	}

	err := c.conn.Close()

	if c.closeCb != nil {
		c.closeCb(c, nil)
	}

	if c.cancelCtx != nil {
		c.cancelFunc()
	}

	return err
}

func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func NewConn(conn net.Conn) *Conn {
	return &Conn{conn: conn}
}
