package socket_client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"lorawan-master/comn/config"
	"lorawan-master/library/log"
	"net/http"
	"sync"
)

type SocketClient struct {
	Conn            *websocket.Conn
	Url             string
	Token           string
	RequestHeader   http.Header
	sendMu          *sync.Mutex // Prevent "concurrent write to websocket connection"
	receiveMu       *sync.Mutex
	OnTextMessage   func(message string, socket SocketClient)
	OnBinaryMessage func(data []byte, socket SocketClient)
	OnConnected     func(socket SocketClient)
	OnConnectError  func(err error, socket SocketClient)
	OnDisconnected  func(err error, socket SocketClient)
	IsConnected     bool
}

func NewSocket(url, token string) SocketClient {
	return SocketClient{
		Url:           url,
		Token:         token,
		RequestHeader: http.Header{},
		sendMu:        &sync.Mutex{},
		receiveMu:     &sync.Mutex{},
	}
}

func (socket *SocketClient) Connect() {
	cert, err := tls.LoadX509KeyPair(config.ApiAuthConfig["certificate"]["tlsCert"], config.ApiAuthConfig["certificate"]["tlsKey"])
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	rawCACert, err := ioutil.ReadFile(config.ApiAuthConfig["certificate"]["caCert"])
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rawCACert)

	websocket.DefaultDialer.TLSClientConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	tokenBear := fmt.Sprintf("Bearer, %s", socket.Token)
	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", tokenBear)

	socket.Conn, _, err = websocket.DefaultDialer.Dial(socket.Url, header)
	if err != nil {
		log.Fatalf("Error while connecting to server: %v" + err.Error())
		socket.IsConnected = false
		if socket.OnConnectError != nil {
			socket.OnConnectError(err, *socket)
		}
		return
	}

	if socket.OnConnected != nil {
		socket.IsConnected = true
		socket.OnConnected(*socket)
	}

	go func() {
		for {
			socket.receiveMu.Lock()
			messageType, message, err := socket.Conn.ReadMessage()
			socket.receiveMu.Unlock()
			if err != nil {
				log.Errorf("read:%v", err)
				return
			}
			//log.Infof("recv: %s", message)

			switch messageType {
			case websocket.TextMessage:
				if socket.OnTextMessage != nil {
					socket.OnTextMessage(string(message), *socket)
				}
			case websocket.BinaryMessage:
				if socket.OnBinaryMessage != nil {
					socket.OnBinaryMessage(message, *socket)
				}
			}
		}
	}()
}

func (socket *SocketClient) send(messageType int, data []byte) error {
	socket.sendMu.Lock()
	err := socket.Conn.WriteMessage(messageType, data)
	socket.sendMu.Unlock()
	return err
}

func (socket *SocketClient) Close() {
	err := socket.send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Errorf("write close: %v", err)
	}
	socket.Conn.Close()
	if socket.OnDisconnected != nil {
		socket.IsConnected = false
		socket.OnDisconnected(err, *socket)
	}
}
