package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	PriorityImmediate     = 10
	PriorityPowerConserve = 5
)

const (
	commandID = 2

	// Items IDs
	deviceTokenItemID            = 1
	payloadItemID                = 2
	notificationIdentifierItemID = 3
	expirationDateItemID         = 4
	priorityItemID               = 5

	// Item lengths
	deviceTokenItemLength            = 32
	notificationIdentifierItemLength = 4
	expirationDateItemLength         = 4
	priorityItemLength               = 1
)

type NotificationResult struct {
	Notif Notification
	Err   Error
}

type Alert struct {
	// Do not add fields without updating the implementation of isZero.
	Body         string   `json:"body,omitempty"`
	Title        string   `json:"title,omitempty"`
	Action       string   `json:"action,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

func (a *Alert) isZero() bool {
	return len(a.Body) == 0 && len(a.LocKey) == 0 && len(a.LocArgs) == 0 && len(a.ActionLocKey) == 0 && len(a.LaunchImage) == 0
}

type APS struct {
	Alert            Alert
	Badge            *int // 0 to clear notifications, nil to leave as is.
	Sound            string
	ContentAvailable int
	URLArgs          []string
	Category         string // requires iOS 8+
}

func (aps APS) serializeForAPNS() map[string]interface{} {
	data := make(map[string]interface{})

	if !aps.Alert.isZero() {
		data["alert"] = aps.Alert
	}
	if aps.Badge != nil {
		data["badge"] = aps.Badge
	}
	if aps.Sound != "" {
		data["sound"] = aps.Sound
	}
	if aps.ContentAvailable != 0 {
		data["content-available"] = aps.ContentAvailable
	}
	if aps.Category != "" {
		data["category"] = aps.Category
	}
	if aps.URLArgs != nil && len(aps.URLArgs) != 0 {
		data["url-args"] = aps.URLArgs
	}

	return data
}

func (aps APS) MarshalJSONForAPNS() ([]byte, error) {

	return json.Marshal(aps.serializeForAPNS())
}

type Payload struct {
	APS APS
	// MDM for mobile device management
	MDM          string
	CustomValues map[string]interface{}
}

func (p *Payload) serializeForAPNS() map[string]interface{} {

	data := make(map[string]interface{})

	for k,v := range p.CustomValues {
		data[k] = v
	}

	if len(p.MDM) != 0 {
		data["mdm"] = p.MDM
	} else {
		data["aps"] = p.APS.serializeForAPNS()
	}

	return data
}

func (p *Payload) MarshalJSONForAPNS() ([]byte, error) {

	return json.Marshal(p.serializeForAPNS())
}


func (p *Payload) SetCustomValue(key string, value interface{}) error {
	if key == "aps" {
		return errors.New("cannot assign a custom APS value in payload")
	}

	p.CustomValues[key] = value

	return nil
}

type Notification struct {
	ID          string
	DeviceToken string
	Identifier  uint32
	Expiration  *time.Time
	Priority    int
	Payload     *Payload
}

func NewNotification() Notification {
	return Notification{Payload: NewPayload()}
}

func NewPayload() *Payload {
	return &Payload{CustomValues: map[string]interface{}{}}
}

func (n Notification) ToBinary() ([]byte, error) {
	b := []byte{}

	binTok, err := hex.DecodeString(n.DeviceToken)
	if err != nil {
		return b, fmt.Errorf("convert token to hex error: %s", err)
	}

	var j []byte
	if n.Payload != nil {
		j, _ = n.Payload.MarshalJSONForAPNS()
	}

	buf := bytes.NewBuffer(b)

	// Token
	binary.Write(buf, binary.BigEndian, uint8(deviceTokenItemID))
	binary.Write(buf, binary.BigEndian, uint16(deviceTokenItemLength))
	binary.Write(buf, binary.BigEndian, binTok)

	// Payload
	binary.Write(buf, binary.BigEndian, uint8(payloadItemID))
	binary.Write(buf, binary.BigEndian, uint16(len(j)))
	binary.Write(buf, binary.BigEndian, j)

	// Identifier
	binary.Write(buf, binary.BigEndian, uint8(notificationIdentifierItemID))
	binary.Write(buf, binary.BigEndian, uint16(notificationIdentifierItemLength))
	binary.Write(buf, binary.BigEndian, uint32(n.Identifier))

	// Expiry
	binary.Write(buf, binary.BigEndian, uint8(expirationDateItemID))
	binary.Write(buf, binary.BigEndian, uint16(expirationDateItemLength))
	if n.Expiration == nil {
		binary.Write(buf, binary.BigEndian, uint32(0))
	} else {
		binary.Write(buf, binary.BigEndian, uint32(n.Expiration.Unix()))
	}

	// Priority
	binary.Write(buf, binary.BigEndian, uint8(priorityItemID))
	binary.Write(buf, binary.BigEndian, uint16(priorityItemLength))
	binary.Write(buf, binary.BigEndian, uint8(n.Priority))

	framebuf := bytes.NewBuffer([]byte{})
	binary.Write(framebuf, binary.BigEndian, uint8(commandID))
	binary.Write(framebuf, binary.BigEndian, uint32(buf.Len()))
	binary.Write(framebuf, binary.BigEndian, buf.Bytes())

	return framebuf.Bytes(), nil
}
