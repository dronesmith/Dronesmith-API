package dronedp

import (
  "bytes"
  "encoding/binary"
  "encoding/json"
  "errors"
  // "log"

  "dronemanager/crc16"
  "mavlink/parser"
)

type OP uint8

const (
  // TODO - actual TLS or hmac authentication here. Keep this out of code. Not actually 'secure.'
  // SECURE_KEY []byte = []byte{
  //   0xd7, 0xe6, 0xaf, 0x0b, 0x14, 0x90, 0x7e, 0xa5, 0x0a, 0xfd, 0xe8, 0xbb,
  //   0x57, 0x4f, 0x3d, 0x99, 0x81, 0x88, 0xd9, 0xf5, 0x1b, 0x90, 0x7d, 0x3d,
  //   0x44, 0xe7, 0x94, 0xe3, 0x30, 0xf0, 0x55, 0xd9,
  // }

  // Ops
  OP_STATUS OP = 0x10
  OP_CODE OP = 0x11
  OP_TERMINAL OP = 0x12
  OP_MAVLINK_TEXT OP = 0xFD
  OP_MAVLINK_BIN OP = 0xFE
)


type Msg struct {
  Op      OP
  Session uint32
  Data    interface{}
}

type StatusMsg struct {
  Op        string          `json:"op"`

  Serial    string          `json:"serialId,omitempty"`
  SimId     string          `json:"simid,omitempty"`
  Email     string          `json:"email,omitempty"`
  Password  string          `json:"password,omitempty"`
  Drone     map[string]interface{}     `json:"drone,omitempty"`
  User      string          `json:"user,omitempty"`
  Terminal  bool            `json:"terminal,omitempty"`
}

type CodeMsg struct {
  Op        string  `json:"op"`
  Msg       string  `json:"msg"`
  Status    int     `json:"status"`
}

type TerminalMsg struct {
  Op        string        `json:"op"`
  Status    bool          `json:"status"`
  Msg       TerminalInfo  `json:"msg"`
}

type TerminalInfo struct {
  User      string `json:"uname"`
  Pass      string `json:"pass"`
  Port      int    `json:"port"`
  Url       string `json:"url"`
}

// 859132162
// 19703425322537218

// =============================================================================
// GenerateMsg
// =============================================================================
func GenerateMsg(opCode OP, session uint32, data interface{}) ([]byte, error) {
  var err error = nil
  var payload []byte

  switch opCode {
    // Binary encoded messages for storing flight data.
  case OP_MAVLINK_BIN:
    packet := data.([]byte)
    payload = make([]byte, len(packet))
    copy(payload, packet)

    // Status and MAVLINK messages contain are json encoded
  case OP_CODE:
    fallthrough
  case OP_MAVLINK_TEXT:
    fallthrough
  case OP_STATUS:
    payload, err = json.Marshal(data)
    if err != nil {
      return nil, err
    }

  default:
    return nil, errors.New("D2P.Gen: Unknown Op code.")
  }

  buf := bytes.NewBuffer(make([]byte, 0))

  // session
  {
    seg := make([]byte, 4)
    binary.BigEndian.PutUint32(seg, session)
    _, err = buf.Write(seg)
  }

  // Op
  err = buf.WriteByte(byte(opCode))

  // payload len
  {
    seg := make([]byte, 2)
    binary.BigEndian.PutUint16(seg, uint16(len(payload)))
    _, err = buf.Write(seg)
  }

  // payload data
  _, err = buf.Write(payload)

  // crc
  {
    seg := make([]byte, 2)
    binary.BigEndian.PutUint16(seg, crc16.Crc16(buf.Bytes()))
    _, err = buf.Write(seg)
  }

  return buf.Bytes(), err
}

// =============================================================================
// ParseMsg
// =============================================================================
func ParseMsg(data []byte) (*Msg, error) {
  var err error

  buf := bytes.NewReader(data)
  msg := &Msg{}

  // Check CRC
  dataSize := len(data)
  crcVal := binary.BigEndian.Uint16(data[dataSize-2:dataSize])
  if crc16.Crc16(data[:dataSize-2]) != crcVal {
    return nil, errors.New("D2P.Parse: CRC Error")
  }

  // Session
  slice := make([]byte, 4)
  _, err = buf.Read(slice)
  msg.Session = binary.BigEndian.Uint32(slice)

  // Op code
  var b byte
  b, err = buf.ReadByte()
  msg.Op = OP(b)

  // Length
  slice = make([]byte, 2)
  _, err = buf.ReadAt(slice, 5)
  length := binary.BigEndian.Uint16(slice)

  // Payload
  decoded := make([]byte, length)
  _, err = buf.ReadAt(decoded, 7)

  if err != nil {
    return nil, err
  }

  switch (msg.Op) {
  case OP_MAVLINK_BIN:
    msg.Data = decoded

  case OP_MAVLINK_TEXT:
    msg.Data = &mavlink.Packet{}
    err = json.Unmarshal(decoded, msg.Data)

  case OP_STATUS:
    msg.Data = &StatusMsg{}
    err = json.Unmarshal(decoded, msg.Data)

  case OP_TERMINAL:
    msg.Data = &TerminalMsg{}
    err = json.Unmarshal(decoded, msg.Data)

  case OP_CODE:
    msg.Data = string(decoded[:])

  default:
    return nil, errors.New("D2P.Parse: Unknown Op code.")

  }

  return msg, err
}
