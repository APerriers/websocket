package socket_server

import (
	"errors"
	"github.com/gorilla/websocket"
	"sync"
)

type SocketServer struct {
	//存放websocket连接
	wsConn *websocket.Conn
	//存数据
	inChan chan []byte
	//读数据
	outChan chan []byte

	closeChan chan byte
	//对closeChan关闭上锁
	mutex sync.Mutex
	// chan是否被关闭
	isClosed bool
}

//初始化长连接
func NewSocketServer(wsConn *websocket.Conn) (conn *SocketServer, err error) {
	conn = &SocketServer{
		wsConn:    wsConn,
		inChan:    make(chan []byte, 1000),
		outChan:   make(chan []byte, 1000),
		closeChan: make(chan byte, 1),
	}

	// 启动读协程
	go conn.readLoop()
	// 启动写协程
	go conn.writeLoop()

	return
}

//读数据
func (conn *SocketServer) ReadMessage() (data []byte, err error) {
	select {
	case data = <-conn.inChan:
	case <-conn.closeChan:
		err = errors.New("connection is closed")
	}
	return
}

//写数据
func (conn *SocketServer) WriteMessage(data []byte) (err error) {
	select {
	case conn.outChan <- data:
	case <-conn.closeChan:
		err = errors.New("connection is closed")
	}

	return
}

func (conn *SocketServer) readLoop() {
	var (
		data []byte
		err  error
	)

	for {
		if _, data, err = conn.wsConn.ReadMessage(); err != nil {
			goto ERR
		}

		// 阻塞在这里，等待inChan有空闲的位置
		select {
		case conn.inChan <- data:
		case <-conn.closeChan:
			// closeChan关闭的时候
			goto ERR
		}
	}

ERR:
	conn.Close()
}


func (conn *SocketServer) writeLoop() {
	var (
		data []byte
		err  error
	)

	for {
		select {
		case data = <-conn.outChan:
		case <-conn.closeChan:
			goto ERR
		}

		data = <-conn.outChan
		if err = conn.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
			goto ERR
		}
	}

ERR:
	conn.Close()
}

func (conn *SocketServer) Close() {
	// 线程安全，可重入的Close
	conn.wsConn.Close()

	// 这一行代码只执行一次
	conn.mutex.Lock()
	if !conn.isClosed {
		close(conn.closeChan)
		conn.isClosed = true
	}

	conn.mutex.Unlock()
}