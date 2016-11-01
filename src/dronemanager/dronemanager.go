/**
 * Dronesmith API
 *
 * Authors
 *  Geoff Gardner <geoff@dronesmith.io>
 *
 * Copyright (C) 2016 Dronesmith Technologies Inc, all rights reserved.
 * Unauthorized copying of any source code or assets within this project, via
 * any medium is strictly prohibited.
 *
 * Proprietary and confidential.
 */

package dronemanager

import (
  "logger"
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
  KEEN_ENV = "production"
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
  SimId string
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
  id            uint32
  State         string
  Terminal      bool
  Drone         map[string]interface{}
  User          string
  link          SessConn
  lastUpdate    time.Time
  syncCloud     time.Time
  veh           *vehicle.Vehicle
  auth          SessAuth
  terminal      dronedp.TerminalInfo
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
        logger.Error("Error: " , err)
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
        logger.Warn("Session", id, "timeout.")
        logger.Warn("Vehicle <" + dId + "> Offline.")

        logger.CloseLog(dId)
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
	  logger.Error(err)
    panic(err)
  }

  buf := make([]byte, 1024)

  go m.checkTimers()

  logger.Info("Listening for vehicles on", m.port)

  for {
    n,addr,err := m.conn.ReadFromUDP(buf)
    // log.Println("Got a message from", addr, "of size", n)
    if err != nil {
      logger.Error("Error: ",err)
    }

    if n < 8 {
      logger.Warn("Warning: Recevied message too small")
      continue
    }

    if decoded, err := dronedp.ParseMsg(buf[0:n]); err != nil {
      logger.Error(err)
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
  case dronedp.OP_TERMINAL:
    terminalMsg := decoded.Data.(*dronedp.TerminalMsg)
    m.handleStatusTerminal(terminalMsg, addr, decoded.Session)
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
  if resp, err := cloud.RequestDroneInfo(msg.Serial, msg.SimId, msg.Email, msg.Password); err != nil {
    logger.Error("Auth failed:", err)
  } else {

    var userId string
    if resp.User["_id"] != nil {
      userId = resp.User["_id"].(string)
    }

    sessObj := &Session{
      State: "online",
      Drone: resp.Drone,
      User: userId,
      lastUpdate: time.Now(),
      syncCloud: time.Now(),
      link: SessConn{0, m.conn, addr,},
      auth: SessAuth{msg.Email, msg.Password, msg.Serial, msg.SimId},
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
      logger.Warn("Could not build D2P MSG:", err)
    } else {
      if _, err = m.conn.WriteToUDP(msg, addr); err != nil {
        logger.Error("Network error:", err)
      } else {
        logger.Info("New session:", sessObj.id)

        // Create a new Vehicle if it does not already exist.
        if sessObj.veh == nil {
          // Id for API is the same as the mongo Id.
          // TODO add name as well.
          logger.Info("Vehicle Authenticated!")
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
      if resp, err := cloud.RequestDroneInfo(sessObj.auth.Serial, sessObj.auth.SimId, sessObj.auth.Email, sessObj.auth.Pass); err != nil {
        logger.Warn("Warning failed to get new drone metadata:", err)
      } else {
        sessObj.Drone = resp.Drone
      }
      sessObj.syncCloud = time.Now()
    }

    if msg, err := dronedp.GenerateMsg(dronedp.OP_STATUS, sessObj.id, sessObj); err != nil {
      logger.Warn("Could not build D2P MSG:", err)
    } else {
      if _, err = m.conn.WriteToUDP(msg, addr); err != nil {
        logger.Error("Network error:", err)
      }
    }
  }
}

func (m *DroneManager) handleStatusTerminal(msg *dronedp.TerminalMsg, addr *net.UDPAddr, id uint32) {
  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()

  if _, found := m.sessions[id]; found {
    // make sure this is ref so we update the timestamp.
    sessObj := m.sessions[id]

    //{msg: data.msg, status: data.status,
    //  drone: statusObj.drone, session: sess}

    logger.Debug("Got terminal on", id)

    sessObj.terminal = msg.Msg
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

// Remember to update the find pattern in the future.
func (m *DroneManager) UpdateTerminal(id string, enable bool) {
  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()

  k := m.searchVehicle(id)
  if sess, f := m.sessions[k]; f {
    logger.Debug("Setting terminal to", enable, "on", k)
    sess.Terminal = enable

    if !enable {
      // delete terminal info
      sess.terminal = dronedp.TerminalInfo{}
    }
  }
}

func (m *DroneManager) GetTerminal(id string) map[string]interface{} {
  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()

  k := m.searchVehicle(id)
  if sess, f := m.sessions[k]; f {
    m := make(map[string]interface{})
    if sess.terminal.Url != "" && sess.terminal.Port != 0 {
      m["url"] = sess.terminal.Url
      m["port"] = sess.terminal.Port
      return m
    } else {
      return nil
    }
  } else {
    return nil
  }
}

// Find a vehicle. Just a sequential search for now, in the future we
// might need to refactor this.
func (m *DroneManager) searchVehicle(id string) uint32 {
  for k , session := range m.sessions {
    // log.Println(k, session)
    if session.Drone["name"] != nil {
      n := session.Drone["name"].(string)
      if n == id {
        return k
      }

      dId := session.Drone["_id"].(string)
      if dId == id {
        return k
      }
    }
  }

  return 0
}

func (m *DroneManager) GetOnlineVehicles() map[string]interface{} {
  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()

  vehs := make(map[string]interface{})

  for _, session := range m.sessions {
    name := session.Drone["name"].(string)
    if name != "" {
      vehs[name] = session.State
    } else {
      id := session.Drone["_id"].(string)
      vehs[id] = session.State
    }
  }

  return vehs
}

func (m *DroneManager) FindVehicle(id string) *vehicle.Vehicle {
  m.sessionLock.Lock()
  defer m.sessionLock.Unlock()

  if sess, f := m.sessions[m.searchVehicle(id)]; f {
    return sess.veh
  } else {
    return nil
  }

  return nil
}
