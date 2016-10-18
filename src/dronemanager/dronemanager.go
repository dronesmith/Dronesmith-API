package dronemanager

import (
  "log"
  "os"
  "strconv"
  "net"
  "cloud"
  "time"
  "math/rand"
  "utils/keen"
  "sync"
  "dronemanager/dronedp"
  "vehicle"
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

// TODO - remove this from memory, or encrypt it.
type SessAuth struct {
  Email string
  Pass string
  Serial string
}

type DroneManager struct {
  port uint
  keenBatch *keen.BatchClient
  sessions map[uint32]*Session
  sessionLock sync.RWMutex
  conn *net.UDPConn
}

type SessConn struct {
  Id   uint32
  conn *net.UDPConn
  addr *net.UDPAddr
}

//
// Used by the encoder in Vehicle to send messages.
//
func (sw *SessConn) Write(p []byte) (n int, err error) {
  if msg, err := dronedp.GenerateMsg(dronedp.OP_MAVLINK_BIN, sw.Id, p); err != nil {
    return 0, err
  } else {
    return sw.conn.WriteToUDP(msg, sw.addr)
  }
}

type Session struct {
  id      uint32
  State   string
  Drone   map[string]interface{}
  User    string
  link    SessConn
  lastUpdate time.Time
  syncCloud time.Time
  veh *vehicle.Vehicle
  auth    SessAuth
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
        dId := sess.Drone["_id"].(string)
        log.Println("Session", id, "timeout.")
        log.Println("Vehicle <" + dId + "> Offline.")

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

  log.Println("Listening for vehicles on", m.port)

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
  case dronedp.OP_MAVLINK_BIN:
    mavChunk := decoded.Data.([]byte)
    m.handleMavlink(mavChunk, decoded.Session)
  }
}

func (m *DroneManager) handleStatusMessage(msg *dronedp.StatusMsg, addr *net.UDPAddr, session uint32) {
  switch msg.Op {
  case "connect": m.handleStatusConnect(msg, addr)
  case "status": m.handleStatusUpdate(msg, addr, session)
  }
}

func (m *DroneManager) handleStatusConnect(msg *dronedp.StatusMsg, addr *net.UDPAddr) {
  if resp, err := cloud.RequestDroneInfo(msg.Serial, msg.Email, msg.Password); err != nil {
    log.Println("Auth failed:", err)
  } else {

    userId := resp.User["_id"].(string)

    sessObj := &Session{
      State: "online",
      Drone: resp.Drone,
      User: userId,
      lastUpdate: time.Now(),
      syncCloud: time.Now(),
      link: SessConn{0, m.conn, addr,},
      auth: SessAuth{msg.Email, msg.Password, msg.Serial},
    }

    sessObj.genId()

    sessObj.link.Id = sessObj.id

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
        log.Println("New session:", sessObj.id)

        // Create a new Vehicle if it does not already exist.
        if sessObj.veh == nil {
          // Id for API is the same as the mongo Id.
          // TODO add name as well.
          log.Println("Vehicle Authenticated!")
          dId := sessObj.Drone["_id"].(string)
          sessObj.veh = vehicle.NewVehicle(dId, &sessObj.link)
        }
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

    if time.Now().Sub(sessObj.syncCloud) > 60 * time.Second {
      if resp, err := cloud.RequestDroneInfo(sessObj.auth.Serial, sessObj.auth.Email, sessObj.auth.Pass); err != nil {
        log.Println("Warning failed to get new drone metadata:", err)
      } else {
        sessObj.Drone = resp.Drone
      }
      sessObj.syncCloud = time.Now()
    }

    if msg, err := dronedp.GenerateMsg(dronedp.OP_STATUS, sessObj.id, sessObj); err != nil {
      log.Println("Could not build D2P MSG:", err)
    } else {
      if _, err = m.conn.WriteToUDP(msg, addr); err != nil {
        log.Println("Network error:", err)
      }
    }
  }
}

func (m *DroneManager) handleMavlink(chunk []byte, id uint32) {
  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()
  if _, found := m.sessions[id]; found {
    // make sure this is ref so we update the timestamp.
    sessObj := m.sessions[id]
    sessObj.lastUpdate = time.Now()

    // Time to get swchifty
    sessObj.veh.ProcessPacket(chunk)
  }
}

// Find a vehicle. Just a sequential search for now, in the future we
// might need to refactor this.
func (m *DroneManager) FindVehicle(id string) *vehicle.Vehicle {
  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()
  for _ , session := range m.sessions {

    // Check if name matches first.
    if session.Drone["name"] != nil {
      n := session.Drone["name"].(string)
      if n == id {
        return session.veh
      }
    }

    dId := session.Drone["_id"].(string)
    if dId == id {
      return session.veh
    }
  }

  return nil
}
