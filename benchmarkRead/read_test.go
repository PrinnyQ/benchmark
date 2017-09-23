package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	//"runtime"
	"bufio"
	"testing"
	"time"
)

var server *Server
var serverAddr, bufServerAddr string
var connected chan bool = make(chan bool)

type Head struct {
	Len uint16
	Cmd uint16
}
type Server struct {
	reader io.Reader
}

func NewServer() *Server {
	return &Server{}
}

type Client struct {
	conn net.Conn
}

func NewClient(con net.Conn) *Client {
	return &Client{
		conn: con,
	}
}

func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			fmt.Print("rpc.Serve: accept:", err.Error())
			return
		}
		server.reader = conn
		connected <- true
	}
}
func (server *Server) AcceptWithBufio(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			fmt.Print("rpc.Serve: accept:", err.Error())
			return
		}
		server.reader = bufio.NewReader(conn)
		connected <- true
	}
}

func (client *Client) send(data []byte) error {
	n, err := client.conn.Write(data)
	if err != nil {
		fmt.Errorf("write error:%s,n:%d\n", err.Error(), n)
		return err
	}
	return nil
}
func startServer() {
	server = NewServer()
	ln, err := net.Listen("tcp", ":0") // any available address
	if err != nil {
		fmt.Errorf("net.Listen tcp :0: %v", err)
		return
	}
	go server.Accept(ln)
	serverAddr = ln.Addr().String()
}
func startBufioServer() {
	server = NewServer()
	ln, err := net.Listen("tcp", ":0") // any available address
	if err != nil {
		fmt.Errorf("net.Listen tcp :0: %v", err)
		return
	}
	go server.AcceptWithBufio(ln)
	bufServerAddr = ln.Addr().String()
}
func dial(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, time.Millisecond*100)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}

func genPacket(bodyLen int) []byte {
	dataHead := Head{
		Len: uint16(bodyLen),
		Cmd: 1,
	}
	dataSend := make([]byte, bodyLen+4)
	binary.LittleEndian.PutUint16(dataSend[:2], dataHead.Len)
	binary.LittleEndian.PutUint16(dataSend[2:], dataHead.Cmd)
	return dataSend
}

func readFullPacket(reader io.Reader) ([]byte, error) {
	var headBuf [4]byte
	header := &Head{}
	if _, err := io.ReadFull(reader, headBuf[:]); err != nil {
		return nil, err
	}
	header.Len = binary.LittleEndian.Uint16(headBuf[:2])
	header.Cmd = binary.LittleEndian.Uint16(headBuf[2:])
	bodyBuf := make([]byte, int(header.Len))
	if _, err := io.ReadFull(reader, bodyBuf); err != nil {
		return nil, err
	}
	return bodyBuf, nil
}
func benchmarkRead(b *testing.B, f func(), addr *string) {
	b.StopTimer()
	f()
	client, err := dial(*addr)
	if err != nil {
		b.Fatal(err)
	}
	defer client.conn.Close()
	sendMsg := genPacket(10)
	<-connected
	sendGate := make(chan bool)
	recvGate := make(chan bool)
	b.StartTimer()
	go func() {
		for i := 0; i < b.N; i++ {
			err := client.send(sendMsg)
			if err != nil {
				b.Errorf("send error:%q", err.Error())
			}
			recvGate <- true
			<-sendGate
		}

	}()
	for i := 0; i < b.N; i++ {
		<-recvGate
		_, err := readFullPacket(server.reader)
		if err != nil {
			b.Errorf("read full error:%s", err.Error())
		}
		sendGate <- true
	}
}

func BenchmarkReadFromConn(b *testing.B) {
	benchmarkRead(b, startServer, &serverAddr)
}

func BenchmarkReadFromBufio(b *testing.B) {
	benchmarkRead(b, startBufioServer, &bufServerAddr)
}
