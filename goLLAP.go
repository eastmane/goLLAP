/*
Package goLLAP is a library for receiving and sending LLAP messages over serial

http://openmicros.org/index.php/articles/85-llap-lightweight-local-automation-protocol/297-llap-reference

*/
package goLLAP

import (
	"github.com/tarm/serial"
	"io"
	"log"
	s "strings"
	"sync"
	"time"
)

var validMessages = [...]string{"AWAKE", "TMPA", "TMPB", "BATT", "BATTLOW", "SLEEPING", "STARTED", "ERROR", "APVER"}

var sleepingDeviceMessages = make(map[string][]LLAPMessage)

var mapMutex = &sync.Mutex{}

var serialConfig *serial.Config
var serialPort io.ReadWriteCloser

//LLAPMessage is a generic LLAP Message
type LLAPMessage struct {
	DeviceID     string
	Message      string
	Time         time.Time
	MessageType  string
	MessageValue string
}

// NewGoLLAP sets up a new serial connection to the specified port
func NewGoLLAP(serialPortName string) {
	serialConfig = &serial.Config{Name: serialPortName, Baud: 9600}
	var err error
	serialPort, err = serial.OpenPort(serialConfig)
	if err != nil {
		log.Fatal(err)
	}
}

/*
MessageListener listens for incoming LLAP messages, passing them to the llapMessages channel on receipt.

If it gets an AWAKE message it will attempt to send any queued messages for that Device
*/
func MessageListener(llapMessages chan LLAPMessage, sendMessages chan LLAPMessage) {
	var rawMessage string
	buf := make([]byte, 64)
	for {
		var remainingFragment string
		n, _ := serialPort.Read(buf)
		chunk := string(buf[:n])
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
			deviceID := rawMessage[1:3]
			for _, validMsgType := range validMessages {
				if validMsgType == rawMessage[3:3+len(validMsgType)] {
					messageType = validMsgType
					messageValue = s.TrimRight(rawMessage[3+len(validMsgType):12], "-")
				}
			}
			llapMessage := LLAPMessage{DeviceID: deviceID, Message: rawMessage, Time: time.Now(), MessageType: messageType, MessageValue: messageValue}
			llapMessages <- llapMessage
			if messageType == "AWAKE" {
				mapMutex.Lock()
				queuedMessages, exists := sleepingDeviceMessages[deviceID]
				if exists {
					for _, queuedMessage := range queuedMessages {
						sendMessages <- queuedMessage
					}
					delete(sleepingDeviceMessages, deviceID)
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

// MessageSender sends messages sent to the sendMessages channel
func MessageSender(sendMessages chan LLAPMessage) {
	for {
		sendMessage(<-sendMessages)
	}
}

func sendMessage(message LLAPMessage) {
	serialPort.Write([]byte(message.Message))
}

// QueueMessageForSleepingDevice adds a message to sleepingDeviceMessages which are then triggered when MessageListener sees an AWAKE message from the sleeping device. 
func QueueMessageForSleepingDevice(message LLAPMessage) {
	mapMutex.Lock()
	_, exists := sleepingDeviceMessages[message.DeviceID]
	if exists {
		sleepingDeviceMessages[message.DeviceID] = append(sleepingDeviceMessages[message.DeviceID], message)
	} else {
		sleepingDeviceMessages[message.DeviceID] = []LLAPMessage{message}
	}
	mapMutex.Unlock()
}
