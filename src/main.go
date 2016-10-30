
//
// Server level logic for API core. The actual lib can be embedded into
// other applications.
//

package main

import (
  "cloud"
  "flag"
  "logger"
  // "vehicle"
  // "dronemanager"
  "rest"
)


func main() {
  // ipAddr := flag.String("master", "0.0.0.0:14550", "Network address of incoming MAVLink. (UDP)")
  // remoteAddr := flag.String("remote", "", "Network address to send outbound MAVLink to. (UDP)")

  runningPort := flag.Int("httpPort", 8080, "Networking port to serve HTTP on")
  dlport := flag.Int("dscPort", 4002, "Networking port to listen for DS Links")
  cloudAddr := flag.String("cloud", "http://localhost:4000", "Connection to the cloud.")

  flag.Parse()

  logger.Info("API Service Init...")

  // vehicle := vehicle.NewVehicle(*ipAddr, *remoteAddr)
  // defer vehicle.Close()
  // vehicle.Listen()

  cloud.InitCloud(*cloudAddr)

  apiServer := rest.NewRestServer(*runningPort)
  apiServer.Listen(*dlport)
}
