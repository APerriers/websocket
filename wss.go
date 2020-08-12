package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"lorawan-master/app/handler"
	"lorawan-master/app/orm"
	"lorawan-master/comn/config"
	socket2 "lorawan-master/comn/socket_client"
	"lorawan-master/comn/util"
	"lorawan-master/library/log"
	"net/url"
	"os"
	"os/signal"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	//初始化配置文件
	fileName := "config.yml"
	config.InitOption(fileName)

	//获取令牌
	token := handler.UserLogin()

	url := url.URL{Scheme: config.Config.Service.Scheme, Host: config.Config.Service.Host, Path: config.Config.Service.Path}
	socket := socket2.NewSocket(url.String(), token)

	tokenBear := fmt.Sprintf("Bearer, %s", token)
	socket.RequestHeader.Set("Sec-WebSocket-Protocol", tokenBear)

	socket.OnConnected = func(socket socket2.SocketClient) {
		log.Info("Connected to server success")
	}

	socket.OnConnectError = func(err error, socket socket2.SocketClient) {
		log.Fatalf("connect error: %v ", err.Error())
	}

	socket.OnTextMessage = func(message string, socket socket2.SocketClient) {
		//log.Infof("recv: %s", message)
		DataEncode(message)
	}

	socket.OnDisconnected = func(err error, socket socket2.SocketClient) {
		log.Infof("Disconnected from server: %v", err.Error())
		return
	}

	socket.Connect()

	for {
		select {
		case <-interrupt:
			log.Info("interrupt")
			socket.Close()
			return
		}
	}
}

//数据解码
func DataEncode(message string)  {
	received := new(orm.Received)
	json.Unmarshal([]byte(message), &received)

	if received.Result.Type == "uplink" {
		payload := new(orm.Payload)
		json.Unmarshal([]byte(received.Result.PayloadJSON), &payload)
		log.Infof("data: %s", payload.Data)

		uDec, err := base64.URLEncoding.DecodeString(payload.Data)
		fmt.Println(uDec)
		//fmt.Println(uDec[0:2])
		//fmt.Println(uDec[2:4])
		//fmt.Println(uDec[4:5])
		//fmt.Println(uDec[5:6])
		//fmt.Println(uDec[6:])
		if err != nil {
			log.Warnf("base64 DecodeString err: %v", err)
		}

		if len(uDec) > 5 {
			DataParse(uDec)
		}

	}
}

//解析数据
func DataParse(uDec []byte) {
	tag := util.BytesToUint16(uDec[0:2])
	ver := util.BytesToUint16(uDec[2:4])
	crc := util.BytesToUint16(uDec[4:5])
	ln := util.BytesToUint16(uDec[5:6])

	objectJson := orm.ObjectJSON{
		Tag: tag,
		Ver: ver,
		Crc: uint8(crc),
		Len: uint8(ln),
	}

	if tag == 100 || tag == 102 || tag == 106 || tag == 107 || tag == 109 || tag == 110 { //整型int
		objectJson.Data = binary.LittleEndian.Uint16(uDec[6:])
	} else if tag == 105 { //无符号整数型unsigned int
		objectJson.Data = binary.LittleEndian.Uint32(uDec[6:])
	} else if tag == 108 { //浮点型
		objectJson.Data = util.ByteToFloat64(uDec[6:])
	} else { //字符串
		objectJson.Data = string(uDec[6:])
	}

	//data异或处理，校验数据
	crcResult := util.Calc_crc(uDec[6:])
	if crcResult == uint8(crc) {

	} else {
		log.Warnf("tag=%d Calc_crc err : 数据非法", tag)
	}

	fmt.Println(objectJson)
}
