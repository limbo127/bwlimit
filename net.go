// Copyright © 2023 Meroxa, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bwlimit

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Conn is a net.Conn connection that limits the bandwidth of writes and reads.
type Conn struct {
	net.Conn
	net.TCPConn

	reader *Reader
	writer *Writer
}

// NewConn wraps an existing net.Conn and returns a Conn that limits the
// bandwidth of writes and reads.
// A zero value for writeLimitPerSecond or readLimitPerSecond means the
// corresponding action will not have a bandwidth limit.
func NewConn(conn net.Conn, writeLimitPerSecond, readLimitPerSecond Byte) *Conn {
	bwconn := &Conn{
		Conn:   conn,
		reader: NewReader(conn, readLimitPerSecond),
		writer: NewWriter(conn, writeLimitPerSecond),
	}
	fmt.Print("bwconn......................")
	return bwconn
}

func (c *Conn) CloseRead() error {
	return c.Conn.Close()
}

func (c *Conn) CloseWrite() error {
	return c.Conn.Close()
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
// Write will limit the connection bandwidth if a limit is configured. If the
// size of b is bigger than the rate of bytes per second, writes will be split
// into smaller chunks.
func (c *Conn) Write(b []byte) (n int, err error) {
	return c.writer.Write(b)
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
// Read will limit the connection bandwidth if a limit is configured. If the
// size of b is bigger than the rate of bytes per second, reads will be split
// into smaller chunks.
// Note that since it's not known in advance how many bytes will be read, the
// bandwidth can burst up to 2x of the configured limit when reading the first 2
// chunks.
func (c *Conn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c *Conn) SetDeadline(t time.Time) error {
	err := c.Conn.SetDeadline(t)
	if err == nil {
		c.writer.SetDeadline(t)
		c.reader.SetDeadline(t)
	}
	return err
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.Conn.SetWriteDeadline(t)
	if err == nil {
		c.writer.SetDeadline(t)
	}
	return err
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *Conn) SetReadDeadline(t time.Time) error {
	err := c.Conn.SetReadDeadline(t)
	if err == nil {
		c.reader.SetDeadline(t)
	}
	return err
}

// SetWriteBandwidthLimit sets the bandwidth limit for future Write calls and
// any currently-blocked Write call.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
func (c *Conn) SetWriteBandwidthLimit(bytesPerSecond Byte) {
	c.writer.SetBandwidthLimit(bytesPerSecond)
}

// SetReadBandwidthLimit sets the bandwidth limit for future Read calls and any
// currently-blocked Read call.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
func (c *Conn) SetReadBandwidthLimit(bytesPerSecond Byte) {
	c.reader.SetBandwidthLimit(bytesPerSecond)
}

// SetBandwidthLimit sets the read and write bandwidth limits associated with
// the connection. It is equivalent to calling both SetReadBandwidthLimit and
// SetWriteBandwidthLimit.
func (c *Conn) SetBandwidthLimit(bytesPerSecond Byte) {
	c.writer.SetBandwidthLimit(bytesPerSecond)
	c.reader.SetBandwidthLimit(bytesPerSecond)
}

// WriteBandwidthLimit returns the current write bandwidth limit.
func (c *Conn) WriteBandwidthLimit() Byte {
	return c.writer.BandwidthLimit()
}

// ReadBandwidthLimit returns the current read bandwidth limit.
func (c *Conn) ReadBandwidthLimit() Byte {
	return c.reader.BandwidthLimit()
}

// Listener is a net.Listener that limits the bandwidth of the connections it
// creates.
type Listener struct {
	net.Listener

	writeBytesPerSecond Byte
	readBytesPerSecond  Byte
}

// NewListener wraps an existing net.Listener and returns a Listener that limits
// the bandwidth of the connections it creates.
// A zero value for writeLimitPerSecond or readLimitPerSecond means the
// corresponding action will not have a bandwidth limit.
func NewListener(lis net.Listener, writeLimitPerSecond, readLimitPerSecond Byte) *Listener {
	bwlis := &Listener{Listener: lis}
	bwlis.SetWriteBandwidthLimit(writeLimitPerSecond)
	bwlis.SetReadBandwidthLimit(readLimitPerSecond)
	return bwlis
}

// Accept waits for and returns the next connection to the listener.
// It returns a connection with a configured bandwidth limit. Each connection
// tracks its own bandwidth.
func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if conn != nil {
		conn = NewConn(conn, l.writeBytesPerSecond, l.readBytesPerSecond)
	}
	return conn, err
}

// WriteBandwidthLimit returns the current write bandwidth limit.
func (l *Listener) WriteBandwidthLimit() Byte {
	return l.writeBytesPerSecond
}

// ReadBandwidthLimit returns the current read bandwidth limit.
func (l *Listener) ReadBandwidthLimit() Byte {
	return l.readBytesPerSecond
}

// SetWriteBandwidthLimit sets the bandwidth limit for writes on future
// connections opened in Accept. It has no effect on already opened connections.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
func (l *Listener) SetWriteBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		l.writeBytesPerSecond = 0
		return
	}
	l.writeBytesPerSecond = bytesPerSecond
}

// SetReadBandwidthLimit sets the bandwidth limit for reads on future
// connections opened in Accept. It has no effect on already opened connections.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
func (l *Listener) SetReadBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		l.readBytesPerSecond = 0
		return
	}
	l.readBytesPerSecond = bytesPerSecond
}

// Dialer is a net.Dialer that limits the bandwidth of the connections it
// creates.
type Dialer struct {
	*net.Dialer

	writeBytesPerSecond Byte
	readBytesPerSecond  Byte
}

// NewDialer wraps an existing net.Dialer and returns a Dialer that limits
// the bandwidth of the connections it creates.
// A zero value for writeLimitPerSecond or readLimitPerSecond means the
// corresponding action will not have a bandwidth limit.
func NewDialer(d *net.Dialer, writeLimitPerSecond, readLimitPerSecond Byte) *Dialer {
	bwd := &Dialer{Dialer: d}
	bwd.SetWriteBandwidthLimit(writeLimitPerSecond)
	bwd.SetReadBandwidthLimit(readLimitPerSecond)
	return bwd
}

// Dial connects to the address on the named network. It returns a connection
// with the configured bandwidth limits. Each connection tracks its own
// bandwidth.
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	conn, err := d.Dialer.Dial(network, address)
	if conn != nil {
		conn = NewConn(conn, d.writeBytesPerSecond, d.readBytesPerSecond)
	}
	return conn, err
}

// DialContext connects to the address on the named network using the provided
// context. It returns a connection with the configured bandwidth limits.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.Dialer.DialContext(ctx, network, address)
	if conn != nil {
		conn = NewConn(conn, d.writeBytesPerSecond, d.readBytesPerSecond)
	}
	return conn, err
}

// WriteBandwidthLimit returns the current write bandwidth limit.
func (d *Dialer) WriteBandwidthLimit() Byte {
	return d.writeBytesPerSecond
}

// ReadBandwidthLimit returns the current read bandwidth limit.
func (d *Dialer) ReadBandwidthLimit() Byte {
	return d.readBytesPerSecond
}

// SetWriteBandwidthLimit sets the bandwidth limit for writes on future
// connections opened in Accept. It has no effect on already opened connections.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
func (d *Dialer) SetWriteBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		d.writeBytesPerSecond = 0
		return
	}
	d.writeBytesPerSecond = bytesPerSecond
}

// SetReadBandwidthLimit sets the bandwidth limit for reads on future
// connections opened in Accept. It has no effect on already opened connections.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
func (d *Dialer) SetReadBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		d.readBytesPerSecond = 0
		return
	}
	d.readBytesPerSecond = bytesPerSecond
}
