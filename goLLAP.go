package goLLAP

import (
	//"fmt"
	"github.com/tarm/goserial"
	"io"
	"log"
	s "strings"
	"sync"
	"time"
)

var VALIDMESSAGES = [...]string{"AWAKE", "TMPA", "TMPB", "BATT", "BATTLOW", "SLEEPING", "STARTED", "ERROR", "APVER"}

var sleepingDeviceMessages = make(map[string][]LLAPMessage)

var mapMutex = &sync.Mutex{}

var serialConfig *serial.Config
var serialPort io.ReadWriteCloser

type LLAPMessage struct {
	DeviceId     string
	Message      string
	Time         time.Time
	MessageType  string
	MessageValue string
}

func init() {
	serialConfig = &serial.Config{Name: "/dev/ttyAMA0", Baud: 9600}
	var err error
	serialPort, err = serial.OpenPort(serialConfig)
	if err != nil {
		log.Fatal(err)
	}
	//	fmt.Println("serial opened")
}

func MessageListener(llapMessages chan LLAPMessage, sendMessages chan LLAPMessage) {
	var rawMessage string
	buf := make([]byte, 64)
	for {
		var remainingFragment string
		n, _ := serialPort.Read(buf)
		chunk := string(buf[:n])
		//		fmt.Println("Raw: ", chunk)
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
			llapMessage := LLAPMessage{DeviceId: deviceId, Message: rawMessage, Time: time.Now(), MessageType: messageType, MessageValue: messageValue}
			llapMessages <- llapMessage
			if messageType == "AWAKE" {
				//fmt.Println("Seen AWAKE")
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
		} else if a := s.Index(rawMessage, "a"); a > -1 {
			rawMessage = rawMessage[a:]
		}
	}
}

func MessageSender(sendMessages chan LLAPMessage) {
	for {
		sendMessage(<-sendMessages)
	}
}

func sendMessage(message LLAPMessage) {
	//	fmt.Println("Message: ", message.Message)
	serialPort.Write([]byte(message.Message))
}

func QueueMessageForSleepingDevice(message LLAPMessage) {
	mapMutex.Lock()
	_, exists := sleepingDeviceMessages[message.DeviceId]
	if exists {
		sleepingDeviceMessages[message.DeviceId] = append(sleepingDeviceMessages[message.DeviceId], message)
	} else {
		sleepingDeviceMessages[message.DeviceId] = []LLAPMessage{message}
	}
	//	fmt.Println("Queued: ", message.Message)
	mapMutex.Unlock()
}
