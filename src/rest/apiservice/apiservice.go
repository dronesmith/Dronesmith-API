package apiservice

import (
  "log"
  // "fmt"
  "net/http"
  "regexp"
  "encoding/json"

  "cloud"
  "dronemanager"
  // "vehicle"
)

type DroneAPI struct {
  port uint
  manager *dronemanager.DroneManager
  idRgxp *regexp.Regexp
  spltRgxp *regexp.Regexp
}

func NewDroneAPI(port uint) *DroneAPI {
  api := &DroneAPI{}
  api.port = port
  api.manager = dronemanager.NewDroneManager(api.port)

  api.idRgxp = regexp.MustCompile("[a-z0-9]{24}")
  api.spltRgxp = regexp.MustCompile("/")

  go api.manager.Listen()

  return api
}

func (api *DroneAPI) Send404(w *http.ResponseWriter) {
  http.Error(*w, http.StatusText(404), 404)
}

func (api *DroneAPI) Send403(w *http.ResponseWriter) {
  http.Error(*w, http.StatusText(403), 403)
}

func (api *DroneAPI) SendAPIError(err error, w *http.ResponseWriter) {
  (*w).Header().Set("Content-Type", "application/json")
  (*w).WriteHeader(400)
  t := map[string]string {
    "error": err.Error(),
  }
  json.NewEncoder(*w).Encode(t)
}

func (api *DroneAPI) SendAPIJSON(data map[string]interface{}, w *http.ResponseWriter) {
  (*w).Header().Set("Content-Type", "application/json")
  (*w).WriteHeader(200)
  json.NewEncoder(*w).Encode(data)
}

func (api *DroneAPI) Validate(email, key, id string) (found bool, droneInfo map[string]interface{}) {
  if data, err := cloud.RequestAPIGET("/api/drone/" + id, email, key); err != nil {
    return false, nil
  } else {
    return true, data
  }

}

func (api *DroneAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
  log.Println("REQUEST", req.Method, req.URL.Path)

  paths := api.spltRgxp.Split(req.URL.Path, -1)
  email := req.Header.Get("User-Email")
  key := req.Header.Get("User-Key")

  var filteredPath []string
  for _, s := range paths {
    if s != "" {
      filteredPath = append(filteredPath, s)
    }
  }

  // Just drone, send back all drones associated with user.
  if len(filteredPath) < 2 {
    if data, err := cloud.RequestAPIGET("/api/drone/", email, key); err != nil {
      api.SendAPIError(err, &w)
    } else {
      api.SendAPIJSON(data, &w)
    }
    return
  }

  // TODO match with name.
  if !api.idRgxp.MatchString(filteredPath[1]) {
    api.Send404(&w)
    return
  }

  // Make sure user key and email are valid
  var droneData map[string]interface{}
  var found bool
  if found, droneData = api.Validate(email, key, filteredPath[1]); !found {
    api.Send403(&w)
    return
  }

  // Grab vehicle object for "live" data.
  veh := api.manager.FindVehicle(filteredPath[1])

  // If nil, vehicle isn't online.
  if veh == nil {
    droneData["online"] = false
    api.SendAPIJSON(droneData, &w)
    return
  } else {
    droneData["online"] = true
  }

  // No requests, send vehicle information including online status.
  if len(filteredPath) < 3 {
    api.SendAPIJSON(droneData, &w)
    return
  }

  switch filteredPath[2] {

  }


}
