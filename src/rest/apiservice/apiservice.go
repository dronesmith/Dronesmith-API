package apiservice

import (
  "log"
  "fmt"
  "net/http"
  "regexp"
  "encoding/json"
  "time"

  "cloud"
  "dronemanager"
  "vehicle"
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

func (api *DroneAPI) SendAPIJSON(data interface{}, w *http.ResponseWriter) {
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

  // handle GETs
  if req.Method == "GET" {

    // No requests, send vehicle information including online status.
    if len(filteredPath) < 3 {
      api.SendAPIJSON(droneData, &w)
      return
    }

    chunk := veh.Telem()

    switch filteredPath[2] {
    case "info": api.handleTelem("Info", chunk, &w)
    case "status": api.handleTelem("Status", chunk, &w)
    case "gps": api.handleTelem("Gps", chunk, &w)
    case "mode": api.handleTelem("Mode", chunk, &w)
    case "attitude": api.handleTelem("Attitude", chunk, &w)
    case "position": api.handleTelem("Position", chunk, &w)
    case "motors": api.handleTelem("Motors", chunk, &w)
    case "input": api.handleTelem("Input", chunk, &w)
    case "rates": api.handleTelem("Rates", chunk, &w)
    case "target": api.handleTelem("Target", chunk, &w)
    case "sensors": api.handleTelem("Sensors", chunk, &w)
    case "home": api.handleTelem("Home", chunk, &w)
    case "log": api.handleLog(veh, &w)
    default: api.Send404(&w)
    }
  } else if req.Method == "POST" {
    decoder := json.NewDecoder(req.Body)
    var pdata map[string]interface{}
    err := decoder.Decode(&pdata)
    if err != nil {
      api.Send404(&w)
      return
    }
    defer req.Body.Close()

    switch filteredPath[2] {
    case "mode": api.handleModeArm(veh, pdata, &w)
    case "command":
    case "param":
    default: api.Send404(&w)
    }
  }
}

func (api *DroneAPI) handleModeArm(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  doSetArm := false
  doSetMode := false
  arming := false
  mode := ""

  if a, f := postData["arm"]; f {
    doSetArm = true
    arming = a.(bool)
  }

  if m, f := postData["mode"]; f {
    doSetMode = true
    mode = m.(string)
  }

  veh.SetModeAndArm(doSetMode, doSetArm, mode, arming)
  attempts := 0
  data := make(map[string]interface{})
  for {
    if attempts >= 10 {
      break
    }
    time.Sleep(5 * time.Millisecond)

    if veh.GetLastSuccessfulCmd() == 176 {
      data["Status"] = "OK"
      data["Command"] = "Set Vehicle Mode and ARM"
      veh.NullLastSuccessfulCmd()
      api.SendAPIJSON(data, w)
      return
    }

    attempts++
  }

  data["Status"] = "FAIL"
  data["Command"] = "Set Vehicle Mode and ARM"
  api.SendAPIJSON(data, w)
}

func (api *DroneAPI) handleLog(veh *vehicle.Vehicle, w *http.ResponseWriter) {
  data := veh.GetSysLog()

  if data == nil {
    api.SendAPIJSON(make([]string, 1), w)
  } else {
    api.SendAPIJSON(data, w)
  }
}

func (api *DroneAPI) handleTelem(kind string, data map[string]interface{}, w *http.ResponseWriter) {
  val, found := data[kind]

  if found {
    api.SendAPIJSON(val, w)
  } else {
    api.SendAPIError(fmt.Errorf("Could not retrieve " + kind + " object."), w)
  }
}
