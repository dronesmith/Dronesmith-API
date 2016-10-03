package dronemanager

import (
  "log"
  "os"
  "strconv"
  "net"
  "time"
  "math/rand"
  "dronemanager/dronedp"
)

type DroneManager struct {
  port uint
  sessions map[string]Session
}

type Session struct {
  id    uint32
}

func (s *Session) genRandomId() {
  s.id = uint32(rand.Int())
}

func CheckError(err error) {
    if err != nil {
        log.Println("Error: " , err)
        os.Exit(1)
    }
}

func NewDroneManager(port uint) *DroneManager {
  rand.Seed(time.Now().UTC().UnixNano())
  return &DroneManager{
    port,
    make(map[string]Session),
  }
}

func (m *DroneManager) Listen() {
  ServerAddr,err := net.ResolveUDPAddr("udp", ":" + strconv.Itoa(int(m.port)))
  CheckError(err)

  ServerConn, err := net.ListenUDP("udp", ServerAddr)
  CheckError(err)
  defer ServerConn.Close()

  if err != nil {
	   log.Println(err)
     panic(err)
  }

  buf := make([]byte, 1024)

  for {
    n,addr,err := ServerConn.ReadFromUDP(buf)
    if err != nil {
      log.Println("Error: ",err)
    }

    if decoded, err := dronedp.ParseMsg(buf[0:n]); err != nil {
      log.Println(err)
    } else {
      log.Println(addr)
      m.handleMessage(decoded)
    }
  }
}

func (m *DroneManager) handleMessage(decoded *dronedp.Msg) {
  log.Println(decoded)
}
