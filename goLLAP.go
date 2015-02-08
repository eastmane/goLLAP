package goLLAP

import (
	"fmt"
	//	zmq "github.com/pebbe/zmq4"
	"github.com/tarm/goserial"
	"io"
	"log"
	s "strings"
	"sync"
	"time"
)

var VALIDMESSAGES = [...]string{"AWAKE", "TMPA", "TMPB", "BATT", "BATTLOW", "SLEEPING", "STARTED", "ERROR", "APVER"}

var sleepingDeviceMessages = make(map[string][]llapMessage)

var mapMutex = &sync.Mutex{}

var serialConfig *serial.Config
var serialPort io.ReadWriteCloser

type llapMessage struct {
	deviceId     string
	message      string
	time         time.Time
	messageType  string
	messageValue string
}

func init() {
	serialConfig = &serial.Config{Name: "/dev/ttyAMA0", Baud: 9600}
	serialPort, err := serial.OpenPort(serialConfig)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("serial opened")
}

func listen(llapMessages chan llapMessage) {
	var rawMessage string
	buf := make([]byte, 64)
	for {
		var remainingFragment string
		n, _ := serialPort.Read(buf)
		chunk := string(buf[:n])
		fmt.Println("Raw: ", chunk)
		if len(rawMessage) > 0 {
			if 12-len(rawMessage) <= len(chunk) {
				remainingFragment = chunk[12-len(rawMessage):]
				rawMessage += chunk[:12-len(rawMessage)]
			}
		} else if string(buf[0]) == "a" {
			rawMessage = string(buf[:n])
		}
		if len(rawMessage) == 12 {
			var messageType string
			var messageValue string
			deviceId := rawMessage[1:3]
			for _, validMsgType := range VALIDMESSAGES {
				if validMsgType == rawMessage[3:3+len(validMsgType)] {
					messageType = validMsgType
					messageValue = s.TrimRight(rawMessage[3+len(validMsgType):12], "-")
				}
			}
			llapMessage := llapMessage{deviceId: deviceId, message: rawMessage, time: time.Now(), messageType: messageType, messageValue: messageValue}
			llapMessages <- llapMessage
			if messageType == "AWAKE" {
				fmt.Println("Seen AWAKE")
				mapMutex.Lock()
				queuedMessages, exists := sleepingDeviceMessages[deviceId]
				if exists {
					for _, queuedMessage := range queuedMessages {
						sendMessages <- queuedMessage
					}
					delete(sleepingDeviceMessages, deviceId)
				}
				mapMutex.Unlock()
			}
			if len(remainingFragment) > 0 {
				rawMessage = remainingFragment
			} else {
				rawMessage = ""
			}
		}
	}
}

func messageSender(sendMessages chan llapMessage) {
	for {
		sendMessage(<-sendMessages)
	}
}

func sendMessage(message llapMessage) {
	fmt.Println("Message: ", message.message)
	serialPort.Write([]byte(message.message))
}

func queueMessageForSleepingDevice(message llapMessage) {
	mapMutex.Lock()
	_, exists := sleepingDeviceMessages[message.deviceId]
	if exists {
		sleepingDeviceMessages[message.deviceId] = append(sleepingDeviceMessages[message.deviceId], message)
	} else {
		sleepingDeviceMessages[message.deviceId] = []llapMessage{message}
	}
	fmt.Println("Queued: ", message.message)
	mapMutex.Unlock()
}
