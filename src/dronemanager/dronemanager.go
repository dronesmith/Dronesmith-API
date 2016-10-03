package dronemanager

import (
  "log"
  "os"
  "strconv"
  "net"
)

type DroneManager struct {
  port uint
}

func CheckError(err error) {
    if err != nil {
        log.Println("Error: " , err)
        os.Exit(1)
    }
}

func NewDroneManager(port uint) *DroneManager {
  return &DroneManager{
    port,
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
    log.Println("Received ",string(buf[0:n]), " from ",addr)

    if err != nil {
      log.Println("Error: ",err)
    }
  }
}
