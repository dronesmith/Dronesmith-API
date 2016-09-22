
//
// Server level logic for API core. The actual lib can be embedded into
// other applications.
//

package main

import (
  "flag"
  "log"
  "vehicle"

)


func main() {
  ipAddr := flag.String("master", "0.0.0.0:14550", "Network address of incoming MAVLink. (UDP)")
  remoteAddr := flag.String("remote", "", "Network address to send outbound MAVLink to. (UDP)")

  flag.Parse()
  log.SetPrefix("[API] ")

  vehicle := vehicle.NewVehicle(*ipAddr, *remoteAddr)
  defer vehicle.Close()
  vehicle.Listen()
}
