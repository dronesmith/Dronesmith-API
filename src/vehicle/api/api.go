
package api

import (
  "time"
  "sync"
  "log"
  "strconv"
  "math"

  "mavlink/parser"
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
  Signal     uint
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

//
// Main Vehicle API struct
//
type VehicleApi struct {
  id        string
  name      string
  created   time.Time

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

  lock      sync.RWMutex
}

func NewVehicleApi(id string) *VehicleApi {
  api := &VehicleApi{}
  api.id = id
  api.created = time.Now()
  api.lock = sync.RWMutex{}
  return api
}

func (v *VehicleApi) UpdateFromHeartbeat(m *mavlink.Heartbeat) {
  v.lock.Lock()
  defer v.lock.Unlock()

  if !v.status.Online {
    v.info.LastOnline = time.Now()
  }

  v.status.Online = true
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
    v.mode = "Altitude Hold"
  } else if m.CustomMode & 0x00FF0000 == 0x030000 {
    v.mode = "Position Hold"
  } else if m.CustomMode & 0x00FF0000 == 0x050000 {
    v.mode = "Acro"
  } else if m.CustomMode & 0x00FF0000 == 0x060000 {
    v.mode = "Offboard"
  } else if m.CustomMode & 0x00FF0000 == 0x070000 {
    v.mode = "Stabilization"
  } else if m.CustomMode & 0x00FF0000 == 0x080000 {
    v.mode = "RAttitude"
  } else if m.CustomMode & 0x00FF0000 == 0x000100 {
    v.mode = "Auto"
  } else if m.CustomMode & 0xFF000000 == 0x02000000 {
    v.mode = "Takeoff"
  } else if m.CustomMode & 0xFF000000 == 0x03000000 {
    v.mode = "Loiter"
  } else if m.CustomMode & 0xFF000000 == 0x04000000 {
    v.mode = "Mission"
  } else if m.CustomMode & 0xFF000000 == 0x05000000 {
    v.mode = "Return To Landing"
  } else if m.CustomMode & 0xFF000000 == 0x06000000 {
    v.mode = "Land"
  } else if m.CustomMode & 0xFF000000 == 0x07000000 {
    v.mode = "RTGS"
  } else if m.CustomMode & 0xFF000000 == 0x08000000 {
    v.mode = "Follow Target"
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

  log.Println(v.status)
}
