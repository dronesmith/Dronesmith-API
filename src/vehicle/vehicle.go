package vehicle

import (
  "net"
  "log"
  "os"
  "time"

  "mavlink/parser"
  "vehicle/api"
)

type Vehicle struct {
  address       *net.UDPAddr
  connection    *net.UDPConn
  mavlinkReader *mavlink.Decoder
  api           *api.VehicleApi
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

func NewVehicle(address string) *Vehicle {
  var err error
  vehicle := &Vehicle{}

  vehicle.api = api.NewVehicleApi("1")

  vehicle.address, err = net.ResolveUDPAddr("udp", address)
  checkError(err)

  vehicle.connection, err = net.ListenUDP("udp", vehicle.address)
  checkError(err)

  vehicle.mavlinkReader = mavlink.NewDecoder(vehicle.connection)
  return vehicle
}

func (v *Vehicle) Listen() {

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

func (v *Vehicle) processPacket(p *mavlink.Packet) {
  switch p.MsgID {
  case mavlink.MSG_ID_HEARTBEAT:
    var m mavlink.Heartbeat
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromHeartbeat(&m)

  case mavlink.MSG_ID_SYS_STATUS:
    var m mavlink.SysStatus
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromStatus(&m)

  case mavlink.MSG_ID_GPS_RAW_INT:
    var m mavlink.GpsRawInt
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromGps(&m)

  case mavlink.MSG_ID_ATTITUDE:
    var m mavlink.Attitude
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromAttitude(&m)

  case mavlink.MSG_ID_LOCAL_POSITION_NED:
    var m mavlink.LocalPositionNed
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromLocalPos(&m)

  case mavlink.MSG_ID_GLOBAL_POSITION_INT:
    var m mavlink.GlobalPositionInt
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromGlobalPos(&m)

  case mavlink.MSG_ID_SERVO_OUTPUT_RAW:
    var m mavlink.ServoOutputRaw
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromMotors(&m)

  case mavlink.MSG_ID_RC_CHANNELS:
    var m mavlink.RcChannels
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromInput(&m)

  case mavlink.MSG_ID_VFR_HUD:
    var m mavlink.VfrHud
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromVfr(&m)

  case mavlink.MSG_ID_HIGHRES_IMU:
    var m mavlink.HighresImu
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromSensors(&m)

  case mavlink.MSG_ID_ATTITUDE_TARGET:
    var m mavlink.AttitudeTarget
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromAttitudeTarget(&m)

  case mavlink.MSG_ID_POSITION_TARGET_LOCAL_NED:
    var m mavlink.PositionTargetLocalNed
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromLocalTarget(&m)

  case mavlink.MSG_ID_POSITION_TARGET_GLOBAL_INT:
    var m mavlink.PositionTargetGlobalInt
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromGlobalTarget(&m)

  case mavlink.MSG_ID_HOME_POSITION:
    var m mavlink.HomePosition
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromHome(&m)

  case mavlink.MSG_ID_EXTENDED_SYS_STATE:
    var m mavlink.ExtendedSysState
    err := m.Unpack(p)
    mavParseError(err)
    v.api.UpdateFromExtSys(&m)
  }
}
