
package api

import (
  "time"
  "sync"
  "logger"
  "fmt"
  // "encoding/hex"
  "strconv"
  "math"

  "mavlink/parser"
  "utils"
)

//
// Endpoint: /drone/:name/info
//
type Info struct {
  Type        string
  Firmware    string
  Protocol    string
  LastUpdate  time.Time
  LastOnline  time.Time
}

//
// Endpoint: /drone/:name/status
//
type Status struct {
  Online      bool
  Armed       bool
  State       string
  VTOLMode    string
  InAir       bool
  Power       uint // 0-100 percent
}

//
// Endpoint /drone/:name/mode
//
type Mode string

//
// Endpoint /drone/:name/gps
//
type Gps struct {
  Satellites   uint
  Latitude     float32
  Longitude    float32
  Altitude     float32
}

//
// Endpoint /drone/:name/attitude
//
type Attitude struct {
  Roll        float32
  Pitch       float32
  Yaw         float32
}

//
// Endpoint /drone/:name/position
//
type Position struct {
  X         float32
  Y         float32
  Z         float32
  Latitude  float32
  Longitude float32
  Altitude  float32
  Heading   float32
}

//
// Endpoint /drone/:name/motors
//

//
// Endpoint /drone/:name/input
//
type Input struct {
  Channels  [18]uint16
  Signal    uint
  Type      string
}

//
// Endpoint /drone/:name/rates
//
type Rate struct {
  Airspeed      float32
  Groundspeed   float32
  Throttle      uint
  Climb         float32
}

//
// Endpoint /drone/:name/target
//
type Target struct {
  Attitude      [4]float32
  Thrust        float32
  X             float32
  Y             float32
  Z             float32
  Latitude      float32
  Longitude     float32
  Altitude      float32
}

//
// Endpoint /drone/:name/sensors
//
type Sensors struct {
  AccX        float32
  AccY        float32
  AccZ        float32
  GyroX       float32
  GyroY       float32
  GyroZ       float32
  MagX        float32
  MagY        float32
  MagZ        float32
  Baro        float32
  Temp        float32
}

//
// Endpoint /drone/:name/home
//
type Home struct {
  X           float32
  Y           float32
  Z           float32
  Latitude    float32
  Longitude   float32
  Altitude    float32
}

type SubSystem struct {
  Updated time.Time
  Online  bool
}

type Param struct {
  Index    uint
  Value    float32 // Even though params can be different values, we always normalize them to a float.
  Encode   uint8 // used to encode the param for MAVLink
}

type VehicleCommand struct {
  Status    uint
  TimesSent uint
  Command   *mavlink.CommandLong
}

type VehicleLog struct {
  Msg       string
  Time      time.Time
  Level     uint
}


//
// Main Vehicle API struct
//
type VehicleApi struct {
  id        string
  name      string
  created   time.Time

  subSystems map[string]*SubSystem

  info      Info
  status    Status
  mode      Mode
  gps       Gps
  attitude  Attitude
  position  Position
  motors    [8]uint16
  input     Input
  rates     Rate
  target    Target
  sensors   Sensors
  home      Home

  sysId     uint8   // MAVLink Target ID
  fmuId     uint64  // Unique ID generated by FMU
  caps      uint64  // Capbilities Mask
  fmuGit    string  // Git hash for FMU firmware
  gotCaps   bool
  params    map[string]*Param
  totalParams uint
  paramsRequested bool
  paramForceInit bool

  lock      sync.RWMutex
}

func (v *VehicleApi) GetVehicleTelem() map[string]interface{} {
  v.lock.Lock()
  defer v.lock.Unlock()

  telem := make(map[string]interface{})
  // Note that this is a deep copy, to avoid race conditions.
  telem["Info"] = v.info
  telem["Status"] = v.status
  telem["Mode"] = v.mode
  telem["Gps"] = v.gps
  telem["Attitude"] = v.attitude
  telem["Position"] = v.position
  telem["Motors"] = v.motors
  telem["Input"] = v.input
  telem["Rates"] = v.rates
  telem["Target"] = v.target
  telem["Sensors"] = v.sensors
  telem["Home"] = v.home
  return telem
}

func (v *VehicleApi) GetGlobal() map[string]float32 {
  v.lock.Lock()
  defer v.lock.Unlock()

  telem := make(map[string]float32)
  telem["Altitude"] = v.position.Altitude
  telem["Longitude"] = v.position.Longitude
  telem["Latitude"] = v.position.Latitude
  return telem
}

func (v *VehicleApi) GetHome() map[string]float32 {
  v.lock.Lock()
  defer v.lock.Unlock()

  telem := make(map[string]float32)
  telem["Altitude"] = v.home.Altitude
  telem["Longitude"] = v.home.Longitude
  telem["Latitude"] = v.home.Latitude
  return telem
}

func (v *VehicleApi) GetMASLAlt() float32 {
  v.lock.Lock()
  defer v.lock.Unlock()
  return v.target.Altitude
}

func NewVehicleApi(id string) *VehicleApi {
  logger.DroneLog(id, "Vehicle <" + id + "> Init")
  api := &VehicleApi{}
  api.id = id
  api.sysId = 0
  api.created = time.Now()
  api.lock = sync.RWMutex{}
  api.subSystems = make(map[string]*SubSystem)
  api.params = make(map[string]*Param)
  api.totalParams = 0
  api.paramsRequested = false
  api.paramForceInit = false
  return api
}

func (v *VehicleApi) AddSubSystem(name string) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.subSystems[name] = &SubSystem{time.Now(), false}
}

func (v *VehicleApi) UpdateSubSystem(name string) error {
  v.lock.Lock()
  defer v.lock.Unlock()

  subsystem, found := v.subSystems[name]
  if !found {
    return fmt.Errorf("No subsystem found")
  } else {
    if !subsystem.Online {
      subsystem.Online = true
      logger.DroneLog(v.id, "Subsystem", name, "online.")
    }
    subsystem.Updated = time.Now()
    return nil
  }
}

func (v *VehicleApi) IsArmed() bool {
  v.lock.Lock()
  defer v.lock.Unlock()
  return v.status.Armed;
}

func (v *VehicleApi) Mode() string {
  v.lock.Lock()
  defer v.lock.Unlock()
  return string(v.mode);
}

func (v *VehicleApi) CheckSubSystems() {
  v.lock.Lock()
  defer v.lock.Unlock()

  for name, subsystem := range v.subSystems {
    if (subsystem.Online) && (time.Now().Sub(subsystem.Updated) > 5 * time.Second) {
      subsystem.Online = false
      logger.DroneLog(v.id, "Subsystem", name, "offline.")
    }
  }
}

//
// Telem messages
//

func (v *VehicleApi) SysOnline() bool {
  v.lock.RLock()
  defer v.lock.RUnlock()
  return v.status.Online
}

func (v *VehicleApi) SysGotCaps() bool {
  v.lock.RLock()
  defer v.lock.RUnlock()

  return v.gotCaps
}

func (v *VehicleApi) Scrub() {
  v.lock.Lock()
  defer v.lock.Unlock()

  v.sysId = 0
  v.fmuId = 0
  v.caps = 0
  v.fmuGit = ""
  v.gotCaps = false

  for k := range v.params {
    delete(v.params, k)
  }
  v.totalParams = 0
  v.paramsRequested = false
}

func (v *VehicleApi) CheckSysOnline() {
  v.lock.Lock()
  defer v.lock.Unlock()

  if v.status.Online && (time.Now().Sub(v.info.LastUpdate) > 5 * time.Second) {
    v.status.Online = false
    logger.DroneLog(v.id, "FMU Offline")
  }
}

func (v *VehicleApi) UpdateFromHeartbeat(m *mavlink.Heartbeat) {
  v.lock.Lock()
  defer v.lock.Unlock()

  if !v.status.Online {
    v.info.LastOnline = time.Now()
    v.status.Online = true
    logger.DroneLog(v.id, "FMU Online")
  }

  v.info.LastUpdate = time.Now()

  switch m.Type {
  default: fallthrough
  case mavlink.MAV_TYPE_GENERIC:        v.info.Type = "Generic Vehicle"
  case mavlink.MAV_TYPE_FIXED_WING:     v.info.Type = "Fixed Wing"
  case mavlink.MAV_TYPE_QUADROTOR:      v.info.Type = "Quadrotor"
  case mavlink.MAV_TYPE_HEXAROTOR:      v.info.Type = "Hexarotor"
  case mavlink.MAV_TYPE_OCTOROTOR:      v.info.Type = "Octorotor"
  case mavlink.MAV_TYPE_VTOL_DUOROTOR:  fallthrough
  case mavlink.MAV_TYPE_VTOL_QUADROTOR: v.info.Type = "VTOL Tailsitter"
  case mavlink.MAV_TYPE_VTOL_TILTROTOR: v.info.Type = "VTOL Tiltrotor"
  }

  switch m.Autopilot {
  default: fallthrough
  case mavlink.MAV_AUTOPILOT_GENERIC:   v.info.Firmware = "Generic Autopilot"
  case mavlink.MAV_AUTOPILOT_SLUGS:     v.info.Firmware = "SLUGS"
  case mavlink.MAV_AUTOPILOT_ARDUPILOTMEGA: v.info.Firmware = "APM"
  case mavlink.MAV_AUTOPILOT_OPENPILOT: v.info.Firmware = "OpenPilot"
  case mavlink.MAV_AUTOPILOT_PPZ:       v.info.Firmware = "Paparazzi UAV"
  case mavlink.MAV_AUTOPILOT_FP:        v.info.Firmware = "FlexiPilot"
  case mavlink.MAV_AUTOPILOT_PX4:       v.info.Firmware = "PX4"
  case mavlink.MAV_AUTOPILOT_SMACCMPILOT: v.info.Firmware = "SMACCMPilot"
  case mavlink.MAV_AUTOPILOT_AUTOQUAD:  v.info.Firmware = "AutoQuad"
  case mavlink.MAV_AUTOPILOT_ARMAZILA:  v.info.Firmware = "Armazila"
  case mavlink.MAV_AUTOPILOT_AEROB:     v.info.Firmware = "Aerob"
  case mavlink.MAV_AUTOPILOT_ASLUAV:    v.info.Firmware = "ASLUAV"
  }

  v.info.Protocol = "MAVLink v" + strconv.Itoa(int(m.MavlinkVersion))

  switch m.SystemStatus {
  default: fallthrough
  case mavlink.MAV_STATE_UNINIT:        v.status.State = "Unknown"
  case mavlink.MAV_STATE_BOOT:          v.status.State = "Initializing"
  case mavlink.MAV_STATE_CALIBRATING:   v.status.State = "Calibrating"
  case mavlink.MAV_STATE_STANDBY:       v.status.State = "Standby"
  case mavlink.MAV_STATE_ACTIVE:        v.status.State = "Active"
  case mavlink.MAV_STATE_CRITICAL:      v.status.State = "Failsafe"
  case mavlink.MAV_STATE_EMERGENCY:     v.status.State = "Mayday"
  case mavlink.MAV_STATE_POWEROFF:      v.status.State = "Powering Down"
  }

  if v.status.State == "Active" {
    v.status.Armed = true
  } else {
    v.status.Armed = false
  }

  if m.CustomMode & 0x00FF0000 == 0x010000 {
    v.mode = "Manual"
  } else if m.CustomMode & 0x00FF0000 == 0x020000 {
    v.mode = "Altitude"
  } else if m.CustomMode & 0x00FF0000 == 0x030000 {
    v.mode = "Position"
  } else if m.CustomMode & 0x00FF0000 == 0x050000 {
    v.mode = "Acro"
  } else if m.CustomMode & 0x00FF0000 == 0x060000 {
    v.mode = "Offboard"
  } else if m.CustomMode & 0x00FF0000 == 0x070000 {
    v.mode = "Stabilized"
  } else if m.CustomMode & 0x00FF0000 == 0x080000 {
    v.mode = "RAttitude"
  } else if m.CustomMode & 0x00FF0000 == 0x000100 {
    v.mode = "Auto"
  } else if m.CustomMode & 0xFF000000 == 0x02000000 {
    v.mode = "Takeoff"
  } else if m.CustomMode & 0xFF000000 == 0x03000000 {
    v.mode = "Hold"
  } else if m.CustomMode & 0xFF000000 == 0x04000000 {
    v.mode = "Mission"
  } else if m.CustomMode & 0xFF000000 == 0x05000000 {
    v.mode = "RTL"
  } else if m.CustomMode & 0xFF000000 == 0x06000000 {
    v.mode = "Land"
  } else if m.CustomMode & 0xFF000000 == 0x07000000 {
    v.mode = "RTGS"
  } else if m.CustomMode & 0xFF000000 == 0x08000000 {
    v.mode = "Follow"
  } else {
    v.mode = "Unknown Flight Mode"
  }
}

func (v *VehicleApi) UpdateFromStatus(m *mavlink.SysStatus) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.status.Power = uint(m.BatteryRemaining)
}

func (v *VehicleApi) UpdateFromGps(m *mavlink.GpsRawInt) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.gps.Satellites = uint(m.SatellitesVisible)
  v.gps.Altitude = float32(m.Alt) / 1000.0
  v.gps.Latitude = float32(m.Lat) / 1e7
  v.gps.Longitude = float32(m.Lon) / 1e7
}

func (v *VehicleApi) UpdateFromAttitude(m *mavlink.Attitude) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.attitude.Roll = m.Roll * (180.0 / math.Pi)
  v.attitude.Pitch = m.Pitch * (180.0 / math.Pi)
  v.attitude.Yaw = m.Yaw * (180.0 / math.Pi)
}

func (v *VehicleApi) UpdateFromLocalPos(m *mavlink.LocalPositionNed) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.position.X = m.X
  v.position.Y = m.Y
  v.position.Z = m.Z
}

func (v *VehicleApi) UpdateFromGlobalPos(m *mavlink.GlobalPositionInt) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.position.Latitude = float32(m.Lat) / 1e7
  v.position.Longitude = float32(m.Lon) / 1e7
  v.position.Altitude = float32(m.RelativeAlt) / 1000.0
  v.position.Heading = float32(m.Hdg) / 100.0
}

func (v *VehicleApi) UpdateFromMotors(m *mavlink.ServoOutputRaw) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.motors[0] = m.Servo1Raw
  v.motors[1] = m.Servo2Raw
  v.motors[2] = m.Servo3Raw
  v.motors[3] = m.Servo4Raw
  v.motors[4] = m.Servo5Raw
  v.motors[5] = m.Servo6Raw
  v.motors[6] = m.Servo7Raw
  v.motors[7] = m.Servo8Raw
}

func (v *VehicleApi) UpdateFromInput(m *mavlink.RcChannels) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.input.Channels[0] = m.Chan1Raw
  v.input.Channels[1] = m.Chan2Raw
  v.input.Channels[2] = m.Chan3Raw
  v.input.Channels[3] = m.Chan4Raw
  v.input.Channels[4] = m.Chan5Raw
  v.input.Channels[5] = m.Chan6Raw
  v.input.Channels[6] = m.Chan7Raw
  v.input.Channels[7] = m.Chan8Raw
  v.input.Channels[8] = m.Chan9Raw
  v.input.Channels[9] = m.Chan10Raw
  v.input.Channels[10] = m.Chan11Raw
  v.input.Channels[11] = m.Chan12Raw
  v.input.Channels[12] = m.Chan13Raw
  v.input.Channels[13] = m.Chan14Raw
  v.input.Channels[14] = m.Chan15Raw
  v.input.Channels[15] = m.Chan16Raw
  v.input.Channels[16] = m.Chan17Raw
  v.input.Channels[17] = m.Chan18Raw
  v.input.Signal = uint(m.Rssi)
  v.input.Type = "Radio Control"
}

func (v *VehicleApi) UpdateFromVfr(m *mavlink.VfrHud) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.rates.Airspeed = m.Airspeed
  v.rates.Groundspeed = m.Groundspeed
  v.rates.Throttle = uint(m.Throttle)
  v.rates.Climb = m.Climb
}

func (v *VehicleApi) UpdateFromSensors(m *mavlink.HighresImu) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.sensors.AccX = m.Xacc
  v.sensors.AccY = m.Yacc
  v.sensors.AccZ = m.Zacc
  v.sensors.GyroX = m.Xgyro
  v.sensors.GyroY = m.Ygyro
  v.sensors.GyroZ = m.Zgyro
  v.sensors.MagX = m.Xmag
  v.sensors.MagY = m.Ymag
  v.sensors.MagZ = m.Zmag
  v.sensors.Baro = m.PressureAlt
  v.sensors.Temp = m.Temperature
}

func (v *VehicleApi) UpdateFromAttitudeTarget(m *mavlink.AttitudeTarget) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.target.Attitude[0] = m.Q[0]
  v.target.Attitude[1] = m.Q[1]
  v.target.Attitude[2] = m.Q[2]
  v.target.Attitude[3] = m.Q[3]
  v.target.Thrust = m.Thrust
}

func (v *VehicleApi) UpdateFromLocalTarget(m *mavlink.PositionTargetLocalNed) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.target.X = m.X
  v.target.Y = m.Y
  v.target.Z = m.Z
}

func (v *VehicleApi) UpdateFromGlobalTarget(m *mavlink.PositionTargetGlobalInt) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.target.Latitude = float32(m.LatInt) / 1e7
  v.target.Longitude = float32(m.LonInt) / 1e7
  v.target.Altitude = m.Alt
}

func (v *VehicleApi) UpdateFromHome(m *mavlink.HomePosition) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.home.X = m.X
  v.home.Y = m.Y
  v.home.Z = m.Z
  v.home.Latitude = float32(m.Latitude) / 1e7
  v.home.Longitude = float32(m.Longitude) / 1e7
  v.home.Altitude = float32(m.Altitude) / 1000
}

func (v *VehicleApi) UpdateFromExtSys(m *mavlink.ExtendedSysState) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  switch m.VtolState {
  default: fallthrough
  case mavlink.MAV_VTOL_STATE_UNDEFINED: v.status.VTOLMode = "Not a VTOL vehicle"
  case mavlink.MAV_VTOL_STATE_TRANSITION_TO_FW: v.status.VTOLMode = "Transition to Fixed Wing"
  case mavlink.MAV_VTOL_STATE_TRANSITION_TO_MC: v.status.VTOLMode = "Transition to Multirotor"
  case mavlink.MAV_VTOL_STATE_MC: v.status.VTOLMode = "Multirotor"
  case mavlink.MAV_VTOL_STATE_FW: v.status.VTOLMode = "Fixed Wing"
  }

  switch m.LandedState {
  default: fallthrough
  case mavlink.MAV_LANDED_STATE_ON_GROUND: fallthrough
  case mavlink.MAV_LANDED_STATE_UNDEFINED: v.status.InAir = false
  case mavlink.MAV_LANDED_STATE_IN_AIR: v.status.InAir = true
  }
}

//
// Command and control
//

func (v *VehicleApi) GetSystemId() uint8 {
  v.lock.RLock()
  defer v.lock.RUnlock()
  return v.sysId
}

func (v *VehicleApi) SetSystemId(id uint8) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.sysId = id
}

func (v *VehicleApi) PackComandLong(Op uint16, params [7]float32) *mavlink.CommandLong {
  c := &mavlink.CommandLong{
    Param1: params[0],
    Param2: params[1],
    Param3: params[2],
    Param4: params[3],
    Param5: params[4],
    Param6: params[5],
    Param7: params[6],
    Command: Op,
    TargetSystem: v.GetSystemId(),
    TargetComponent: 0,
    Confirmation: 0,
  }

  return c
}

func (v *VehicleApi) RequestVehicleInfo() *mavlink.CommandLong {
  return v.PackComandLong(
    mavlink.MAV_CMD_REQUEST_AUTOPILOT_CAPABILITIES,
    [7]float32{1.0})
}

func (v *VehicleApi) UpdateFromAutopilotVersion(m *mavlink.AutopilotVersion) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  v.caps = m.Capabilities
  v.fmuId = m.Uid

  // decode version tag
  {
    major := m.FlightSwVersion & 0xFF000000 >> 24
    minor := m.FlightSwVersion & 0x00FF0000 >> 16
    patch := m.FlightSwVersion & 0x0000FF00 >> 8
    git := m.FlightSwVersion & 0x000000FF
    v.fmuGit = strconv.Itoa(int(major)) + "." + strconv.Itoa(int(minor)) + "." + strconv.Itoa(int(patch)) + "-" + strconv.Itoa(int(git))
  }

  if !v.gotCaps {
    v.PrintCapabilities()
  }

  v.gotCaps = true
}

func (v *VehicleApi) CheckCapability(cap uint64) bool {
  return (v.caps & cap) > 0
}

func (v *VehicleApi) PrintCapabilities() {
  logger.DroneLog(v.id, "[WELCOME TO DSC]")
  logger.DroneLog(v.id, "Comms Protocol:", v.info.Protocol)
  logger.DroneLog(v.id, "Vehicle Configuration:", v.info.Type)
  logger.DroneLog(v.id, "Firmware:", v.info.Firmware)
  logger.DroneLog(v.id, "Version:", v.fmuGit)

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_MISSION_FLOAT) {
    logger.DroneLog(v.id, "\tCOMMAND LONG SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_PARAM_FLOAT) {
    logger.DroneLog(v.id, "\tFLOAT PARAMS SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_MISSION_INT) {
    logger.DroneLog(v.id, "\tMISSION INT SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_COMMAND_INT) {
    logger.DroneLog(v.id, "\tCOMMAND INT SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_PARAM_UNION) {
    logger.DroneLog(v.id, "\tUNION PARAMS SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_FTP) {
    logger.DroneLog(v.id, "\tFTP FROM NONVOLATILE STORAGE SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_SET_ATTITUDE_TARGET) {
    logger.DroneLog(v.id, "\tATTITUDE TARGET SETPOINTS SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_SET_POSITION_TARGET_LOCAL_NED) {
    logger.DroneLog(v.id, "\tLOCAL POSITION TARGET SETPOINTS SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_SET_POSITION_TARGET_GLOBAL_INT) {
    logger.DroneLog(v.id, "\tGLOBAL POSITION TARGET SETPOINTS SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_TERRAIN) {
    logger.DroneLog(v.id, "\tTERRAIN ESTIMATION SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_SET_ACTUATOR_TARGET) {
    logger.DroneLog(v.id, "\tMOTOR TARGET SETPOINTS SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_FLIGHT_TERMINATION) {
    logger.DroneLog(v.id, "\tFLIGHT TERMINATION SUPPORTED")
  }

  if v.CheckCapability(mavlink.MAV_PROTOCOL_CAPABILITY_COMPASS_CALIBRATION) {
    logger.DroneLog(v.id, "\tCOMPASS CALIBRATION SUPPORTED")
  }
}

func (v *VehicleApi) UpdateFromAck(m *mavlink.CommandAck, queue *utils.PQueue) {
  if queue.Size() > 0 {
    cmdInt, pri := queue.Head()
    cmd := cmdInt.(*VehicleCommand)

    // Only update if they are the same command.
    if pri == int(m.Command) {
      cmd.Status = uint(m.Result)
    }
  }

  switch m.Result {
  case mavlink.MAV_RESULT_ACCEPTED:
    logger.DroneLog(v.id, "Command Accepted:", m.Command)
  case mavlink.MAV_RESULT_TEMPORARILY_REJECTED:
    logger.DroneLog(v.id, "Command Rejected:", m.Command)
  case mavlink.MAV_RESULT_DENIED:
    logger.DroneLog(v.id, "Command Can not be completed:", m.Command)
  default: fallthrough
  case mavlink.MAV_RESULT_UNSUPPORTED:
    logger.DroneLog(v.id, "Command Unknown:", m.Command)
  case mavlink.MAV_RESULT_FAILED:
    logger.DroneLog(v.id, "Tried to execute command, but failed", m.Command)
  }
}

func (v *VehicleApi) PackAttPosMocap(q [4]float32, x, y, z float32) *mavlink.AttPosMocap {
  c := &mavlink.AttPosMocap{
    Q: q, X: x, Y: y, Z: z,
  }
  return c
}

//
// Params
//

func (v *VehicleApi) RequestParamsList() *mavlink.ParamRequestList {
  return &mavlink.ParamRequestList{
    v.GetSystemId(), 0,
  }
}

func (v *VehicleApi) RequestParam(param uint) *mavlink.ParamRequestRead {
  return &mavlink.ParamRequestRead{
    int16(param),
    v.GetSystemId(), 0,
    [16]byte{0},
  }
}

func (v *VehicleApi) GetParam(param string) (float32, error) {
  v.lock.RLock()
  defer v.lock.RUnlock()

  if p, f := v.params[param]; f {
    return p.Value, nil
  } else {
    return 0.0, fmt.Errorf("Param not found.")
  }
}

func (v *VehicleApi) GetParamIndex(id uint) (float32, error) {
  v.lock.RLock()
  defer v.lock.RUnlock()

  for _, e := range v.params {
    if e.Index == id {
      return e.Value, nil
    }
  }

  return 0.0, fmt.Errorf("Param not found.")
}

func (v *VehicleApi) SetParam(param string, value float32) *mavlink.ParamSet {
  // convert to [16]byte
  var uid [16]byte = [16]byte{0}
  for i := 0; i < len(param); i += 1 {
    uid[i] = param[i]
  }

  var enc uint8
  for k, e := range v.params {
    if k == param {
      enc = e.Encode
    }
  }

  return &mavlink.ParamSet{
    value,
    v.GetSystemId(), 0,
    uid,
    enc,
  }
}

func (v *VehicleApi) ParamsInit() bool {
  v.lock.RLock()
  defer v.lock.RUnlock()
  return v.paramsRequested
}

func (v *VehicleApi) ResetParams() {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.totalParams = 0
  v.paramsRequested = false
  v.paramForceInit = false
  v.params = make(map[string]*Param)
}

func (v *VehicleApi) AllParams() (uint, map[string]float32) {
  v.lock.RLock()
  defer v.lock.RUnlock()

  vals := make(map[string]float32)
  for s, e := range v.params {
    vals[s] = e.Value
  }

  return v.totalParams, vals
}

func (v *VehicleApi) CheckParams() (uint, map[uint]bool) {
  v.lock.RLock()
  defer v.lock.RUnlock()

  gotParams := make(map[uint]bool)

  for _, e := range v.params {
    gotParams[e.Index] = true
  }

  return v.totalParams, gotParams
}

func (v *VehicleApi) ForceParamInit() {
  v.lock.Lock()
  defer v.lock.Unlock()

  v.paramForceInit = true
}

func (v *VehicleApi) ParamForced() bool {
  v.lock.RLock()
  defer v.lock.RUnlock()

  return v.paramForceInit
}

func (v *VehicleApi) UpdateFromParam(m *mavlink.ParamValue) {
  v.lock.Lock()
  defer v.lock.Unlock()
  v.info.LastUpdate = time.Now()

  // Stop sending params request message, firmware knows to send us back all the params.
  if !v.paramsRequested {
    v.paramsRequested = true
  }

  v.totalParams = uint(m.ParamCount)

  // we need to deep copy the param string, to avoid copying over nils
  str := ""
  for _, c := range m.ParamId {
    if c == 0 {
      break
    }
    str += string(c)
  }

  // log.Println(m.ParamIndex, str)

  v.params[str] = &Param{
    uint(m.ParamIndex),
    m.ParamValue,
    m.ParamType,
  }
}
