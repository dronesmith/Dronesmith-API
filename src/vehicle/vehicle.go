package vehicle

import (
  "net"
  "log"
  "os"
  "time"

  "mavlink/parser"
  "vehicle/api"
)

type VehicleState int
const (
  INIT VehicleState = iota
  GETCAPS
  GETPARAMS
  CONNECTED
  RECORDING
  REPLAY
)

type Vehicle struct {
  address       *net.UDPAddr
  connection    *net.UDPConn
  mavlinkReader *mavlink.Decoder
  mavlinkWriter *mavlink.Encoder

  api           *api.VehicleApi
  state         VehicleState
  knownMsgs     map[string]mavlink.Message
  unknownMsgs   map[uint8]*mavlink.Packet
}

func checkError(err error) {
  if err != nil {
    log.Println("Error:" , err)
    os.Exit(1)
  }
}

func mavParseError(err error) {
  if err != nil {
    log.Println("Mavlink failed to parse:", err)
  }
}

func NewVehicle(address, remote string) *Vehicle {
  var err error
  vehicle := &Vehicle{}

  vehicle.api = api.NewVehicleApi("1")
  vehicle.state = INIT
  vehicle.knownMsgs = make(map[string]mavlink.Message)
  vehicle.unknownMsgs = make(map[uint8]*mavlink.Packet)

  vehicle.api.AddSubSystem("GPS")
  vehicle.api.AddSubSystem("Estimator")
  vehicle.api.AddSubSystem("Controller")
  vehicle.api.AddSubSystem("RadioControl")
  vehicle.api.AddSubSystem("Motors")
  vehicle.api.AddSubSystem("OpticalFlow")
  vehicle.api.AddSubSystem("RangeFinder")
  vehicle.api.AddSubSystem("IMU")

  vehicle.address, err = net.ResolveUDPAddr("udp", address)
  checkError(err)

  vehicle.connection, err = net.ListenUDP("udp", vehicle.address)
  checkError(err)

  vehicle.mavlinkReader = mavlink.NewDecoder(vehicle.connection)

  if remote == "" {
    vehicle.mavlinkWriter = mavlink.NewEncoder(vehicle.connection)
  } else {
    var remoteConn net.Conn

    remoteConn, err = net.Dial("udp", remote)
    checkError(err)

    vehicle.mavlinkWriter = mavlink.NewEncoder(remoteConn)
  }

  return vehicle
}

func (v *Vehicle) GetParams() {
  v.sendMAVLink(v.api.RequestParamsList())
}

func (v *Vehicle) GetParam(name string) {

}

func (v *Vehicle) SetParam(name string) {

}

func (v *Vehicle) Listen() {

  // Check systems are online
  go v.checkOnline()

  // Write logic
  go v.stateHandler()

  // Read logic
  for {
    packet, err := v.mavlinkReader.Decode()
    if err != nil {
      log.Println("Parser:", err)
    } else {
      v.processPacket(packet)
    }

    time.Sleep(1 * time.Millisecond)
  }
}

func (v *Vehicle) Close() {
  v.connection.Close()
}

func (v *Vehicle) sendMAVLink(m mavlink.Message) {
  if err := v.mavlinkWriter.Encode(0, 0, m); err != nil {
    log.Println(err)
  }
}

func (v *Vehicle) sysOnlineHandler() {
  // Main system handler if the init was completed.
  log.Println("System online handler.")
}

//
// Basically the init has 3 steps:
// 1, ensure we're online
// 2, got vehicle capabilities
// 3, have all the vehicle params.
// After we've passed these three things, we're good to go.
//
func (v *Vehicle) stateHandler() {
  for {
    online := v.api.SysOnline()
    caps := v.api.SysGotCaps()

    // only do stuff if we're online
    if online {
      if !caps {
        // Get caps
        v.sendMAVLink(v.api.RequestVehicleInfo())
        log.Println("Fetching caps...")
      } else {
        if !v.api.ParamsInit() {
          log.Println("Fetching params...")
          v.GetParams()
        } else {
          if found, missing := v.api.CheckParams(); found {
            // We're fully initialized!
            v.sysOnlineHandler()
          } else if !found {
            // Don't have all of them, invidually request the params we don't have.
            for e := range missing {
              v.sendMAVLink(v.api.RequestParam(uint(e)))
              // wait a teensy bit to give the firmware time to receive
              time.Sleep(2 * time.Millisecond)
            }
          }
        }
      }
    } else {
      // Remove stale data
      // NOTE we purposely keep most of the telemetry data to preserve the drone's
      // last live state. We only remove internal MAVLink information like params
      // and caps.
      v.api.Scrub()
    }

    time.Sleep(200 * time.Millisecond)
  }
}

func (v *Vehicle) checkOnline() {
  for {
    v.api.CheckSysOnline()
    v.api.CheckSubSystems()

    time.Sleep(1 * time.Second)
  }
}

func (v *Vehicle) processPacket(p *mavlink.Packet) {
  if v.api.GetSystemId() == 0 {
    v.api.SetSystemId(p.SysID)
  }

  switch p.MsgID {
  case mavlink.MSG_ID_HEARTBEAT:
    var m mavlink.Heartbeat
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromHeartbeat(&m)
    v.knownMsgs[m.MsgName()] = &m

  case mavlink.MSG_ID_SYS_STATUS:
    var m mavlink.SysStatus
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromStatus(&m)
    v.knownMsgs[m.MsgName()] = &m

  case mavlink.MSG_ID_GPS_RAW_INT:
    var m mavlink.GpsRawInt
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromGps(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("GPS")

  case mavlink.MSG_ID_ATTITUDE:
    var m mavlink.Attitude
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromAttitude(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("Estimator")

  case mavlink.MSG_ID_LOCAL_POSITION_NED:
    var m mavlink.LocalPositionNed
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromLocalPos(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("Estimator")

  case mavlink.MSG_ID_GLOBAL_POSITION_INT:
    var m mavlink.GlobalPositionInt
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromGlobalPos(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("Estimator")

  case mavlink.MSG_ID_SERVO_OUTPUT_RAW:
    var m mavlink.ServoOutputRaw
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromMotors(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("Motors")

  case mavlink.MSG_ID_RC_CHANNELS:
    var m mavlink.RcChannels
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromInput(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("RadioControl")

  case mavlink.MSG_ID_VFR_HUD:
    var m mavlink.VfrHud
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromVfr(&m)
    v.knownMsgs[m.MsgName()] = &m

  case mavlink.MSG_ID_HIGHRES_IMU:
    var m mavlink.HighresImu
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromSensors(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("IMU")

  case mavlink.MSG_ID_ATTITUDE_TARGET:
    var m mavlink.AttitudeTarget
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromAttitudeTarget(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("Controller")

  case mavlink.MSG_ID_POSITION_TARGET_LOCAL_NED:
    var m mavlink.PositionTargetLocalNed
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromLocalTarget(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("Controller")

  case mavlink.MSG_ID_POSITION_TARGET_GLOBAL_INT:
    var m mavlink.PositionTargetGlobalInt
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromGlobalTarget(&m)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("Controller")

  case mavlink.MSG_ID_HOME_POSITION:
    var m mavlink.HomePosition
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromHome(&m)
    v.knownMsgs[m.MsgName()] = &m

  case mavlink.MSG_ID_EXTENDED_SYS_STATE:
    var m mavlink.ExtendedSysState
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromExtSys(&m)
    v.knownMsgs[m.MsgName()] = &m

  case mavlink.MSG_ID_DISTANCE_SENSOR:
    var m mavlink.DistanceSensor
    err := m.Unpack(p)
    mavParseError(err)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("RangeFinder")

  case mavlink.MSG_ID_OPTICAL_FLOW_RAD:
    var m mavlink.OpticalFlowRad
    err := m.Unpack(p)
    mavParseError(err)
    v.knownMsgs[m.MsgName()] = &m
    v.api.UpdateSubSystem("OpticalFlow")

  case mavlink.MSG_ID_COMMAND_ACK:
    var m mavlink.CommandAck
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromAck(&m)
    v.knownMsgs[m.MsgName()] = &m

  case mavlink.MSG_ID_AUTOPILOT_VERSION:
    var m mavlink.AutopilotVersion
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromAutopilotVersion(&m)
    v.knownMsgs[m.MsgName()] = &m

  case mavlink.MSG_ID_PARAM_VALUE:
    var m mavlink.ParamValue
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromParam(&m)
    v.knownMsgs[m.MsgName()] = &m

  default:
    v.unknownMsgs[p.MsgID] = p
  }
}
