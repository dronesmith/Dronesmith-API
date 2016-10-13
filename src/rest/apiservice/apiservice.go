package apiservice

import (
  "log"
  // "fmt"
  "net/http"
  "regexp"

  "dronemanager"
  // "vehicle"
)

//
// Private Regexps. Put up here so we don't compile them on every request.
//
var (
)

type APIService struct {
  port uint
  manager *dronemanager.DroneManager
  idRgxp *regexp.Regexp
  spltRgxp *regexp.Regexp
}

func NewAPIService(port uint) *APIService {
  api := &APIService{}
  api.port = port
  api.manager = dronemanager.NewDroneManager(api.port)

  api.idRgxp = regexp.MustCompile("[a-z0-9]{24}")
  api.spltRgxp = regexp.MustCompile("/")

  go api.manager.Listen()

  return api
}

func (api *APIService) Send404(w *http.ResponseWriter) {
  http.Error(*w, http.StatusText(404), 404)
}

func (api *APIService) ServeHTTP(w http.ResponseWriter, req *http.Request) {
  log.Println("REQUEST", req.Method, req.URL.Path)

  paths := api.spltRgxp.Split(req.URL.Path, -1)

  if paths[0] != "drone" {
    api.Send404(&w)
    return
  }

  // TODO match with name.
  if !api.idRgxp.MatchString(paths[1]) {
    api.Send404(&w)
    return
  }

  veh := api.manager.FindVehicle(paths[1])

  log.Println(veh)

  if veh == nil {
    api.Send404(&w)
    return
  }

  if len(paths) < 3 {
    // No sub API call, just respond with general info about their drone.
  }

  switch paths[2] {

  }

  // fetch drone ID and sub path

  // fmt.Fprintf(w,
  //   "<html><head><title>%s</title></head><body><p>%s</p></body></html>",
  //   "DS API Service",
  //   "API Request.")

}
