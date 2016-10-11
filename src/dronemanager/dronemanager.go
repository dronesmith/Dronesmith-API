package dronemanager

import (
  "log"
  "os"
  "strconv"
  "net"
  "rest"
  "time"
  "math/rand"
  "utils/keen"
  "sync"
  "dronemanager/dronedp"
)

const (
  KEEN_ENV = "testing"
  KEEN_WRITE = "7215fb73a83d556fed9ff4c12b057aec30cdcb75735d56b736f41b74c434fa964fefda477fde39953a6fa6dd095f4fcf3bce1b1d59731c640c9800890bc7a137c242f62361d84d3aa57f2e95009209f921ec65d4130f0bbb838c262b330f3767"
  KEEN_ID = "57a4c674e86170469d49e265"
)

var (
  SessionIds uint32 = 1
)

type DLTracker struct {
  Env string
  Event string
  Session Session
}

type DroneManager struct {
  port uint
  keenBatch *keen.BatchClient
  sessions map[uint32]*Session
  sessionLock sync.RWMutex
  conn *net.UDPConn
}

type Session struct {
  id      uint32
  State   string
  Drone   map[string]interface{}
  User    string
  lastUpdate time.Time
}

func (s *Session) genRandomId() {
  s.id = uint32(rand.Int())
}

func (s *Session) genId() {
  s.id = SessionIds
  SessionIds++
  if SessionIds == 0 {
    SessionIds = 1
  }
}

func CheckError(err error) {
    if err != nil {
        log.Println("Error: " , err)
        os.Exit(1)
    }
}

func NewDroneManager(port uint) *DroneManager {
  rand.Seed(time.Now().UTC().UnixNano())
  keenClient := &keen.Client{WriteKey: KEEN_WRITE, ProjectID: KEEN_ID}
  return &DroneManager{
    port,
    keen.NewBatchClient(keenClient, 10 * time.Second),
    make(map[uint32]*Session),
    sync.RWMutex{},
    nil,
  }
}

func (m *DroneManager) checkTimers() {
  for {
    m.sessionLock.Lock()
    for id, sess := range m.sessions {
      if time.Now().Sub(sess.lastUpdate) > (5 * time.Second) {
        log.Println(id, "Session timeout")

        delete(m.sessions, id)
        m.keenBatch.AddEvent("dronelink", &DLTracker{
          Env: KEEN_ENV,
          Event: "disconnect",
        })
      }
    }
    m.sessionLock.Unlock()
    time.Sleep(5 * time.Second)
  }
}

func (m *DroneManager) Listen() {
  ServerAddr,err := net.ResolveUDPAddr("udp", ":" + strconv.Itoa(int(m.port)))
  CheckError(err)

  m.conn, err = net.ListenUDP("udp", ServerAddr)
  CheckError(err)
  defer m.conn.Close()

  if err != nil {
	  log.Println(err)
    panic(err)
  }

  buf := make([]byte, 1024)

  go m.checkTimers()

  for {
    n,addr,err := m.conn.ReadFromUDP(buf)
    if err != nil {
      log.Println("Error: ",err)
    }

    if decoded, err := dronedp.ParseMsg(buf[0:n]); err != nil {
      log.Println(err)
    } else {
      // Doing this async
      go m.handleMessage(decoded, addr)
    }
  }
}

func (m *DroneManager) handleMessage(decoded *dronedp.Msg, addr *net.UDPAddr) {
  // log.Println(decoded)

  switch decoded.Op {
  case dronedp.OP_STATUS:
    statusMsg := decoded.Data.(*dronedp.StatusMsg)
    m.handleStatusMessage(statusMsg, addr, decoded.Session)
  }
}

func (m *DroneManager) handleStatusMessage(msg *dronedp.StatusMsg, addr *net.UDPAddr, session uint32) {
  switch msg.Op {
  case "connect": m.handleStatusConnect(msg, addr)
  case "status": m.handleStatusUpdate(msg, addr, session)
  }
}

func (m *DroneManager) handleStatusConnect(msg *dronedp.StatusMsg, addr *net.UDPAddr) {
  if resp, err := rest.RequestDroneInfo(msg.Serial, msg.Email, msg.Password); err != nil {
    log.Println("Auth failed:", err)
  } else {

    userId := resp.User["_id"].(string)

    sessObj := &Session{
      State: "online",
      Drone: resp.Drone,
      User: userId,
      lastUpdate: time.Now(),
    }

    sessObj.genId()

    m.sessionLock.Lock()
    defer m.sessionLock.Unlock()
    m.sessions[sessObj.id] = sessObj

    // record keen event
    m.keenBatch.AddEvent("dronelink", &DLTracker{
      Env: KEEN_ENV,
      Event: "connect",
      Session: *sessObj,
    })

    if msg, err := dronedp.GenerateMsg(dronedp.OP_STATUS, sessObj.id, sessObj); err != nil {
      log.Println("Could not build D2P MSG:", err)
    } else {
      if _, err = m.conn.WriteToUDP(msg, addr); err != nil {
        log.Println("Network error:", err)
      } else {
        log.Println("Session Changed:", sessObj.id)
      }
    }
  }
}

func (m *DroneManager) handleStatusUpdate(msg *dronedp.StatusMsg, addr *net.UDPAddr, id uint32) {

  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()
  if _, found := m.sessions[id]; found {
    // make sure this is ref so we update the timestamp.
    sessObj := m.sessions[id]
    sessObj.lastUpdate = time.Now()

    if msg, err := dronedp.GenerateMsg(dronedp.OP_STATUS, sessObj.id, sessObj); err != nil {
      log.Println("Could not build D2P MSG:", err)
    } else {
      if _, err = m.conn.WriteToUDP(msg, addr); err != nil {
        log.Println("Network error:", err)
      }
    }

    // Get payload data if any (TODO)

    // Make command buffer null (TODO)
  }
}
