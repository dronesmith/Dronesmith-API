package apiservice

import (
  "log"
  "fmt"
  "math"
  "net/http"
  "regexp"
  "encoding/json"
  "time"
  "strconv"

  "mavlink/parser"
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
  err := (json.NewEncoder(*w)).Encode(data)
  if err != nil {
    panic(err)
  }
}

func (api *DroneAPI) Validate(email, key, id string) (found bool, droneInfo map[string]interface{}) {
  if data, err := cloud.RequestAPIGET("/api/drone/" + id, email, key); err != nil {
    return false, nil
  } else {
    return true, data
  }

}

func (api *DroneAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
  // Handle panics
  defer func() {
    if r := recover(); r != nil {
      w.WriteHeader(500)
      log.Println("Request paniced!", r)
      fmt.Fprintf(w, "I couldn't parse your request. Make sure you are formating your JSON properly, including types.\n")
    }
  }()

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
    case "param":
      if len(filteredPath) < 4 {
        api.Send404(&w)
      } else {
        api.handleGetSingleParam(veh, filteredPath[3], &w)
      }
    case "params":
      if len(filteredPath) < 4 {
        api.handleGetAllParams(veh, &w)
      } else if filteredPath[3] == "refresh" {
        api.handleRefreshParams(veh, &w)
      }
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
    case "takeoff": api.handleTakeoff(veh, pdata, &w)
    case "input": api.handleInput(veh, pdata, &w)
    case "mode": api.handleModeArm(veh, pdata, &w)
    case "command":api.handleCommand(veh, pdata, &w)
    case "param":
      if len(filteredPath) < 4 {
        api.Send404(&w)
      } else {
        api.handleSetParam(veh, filteredPath[3], pdata, &w)
      }
    case "home": api.handleSetHome(veh, pdata, &w)
    default: api.Send404(&w)
    }
  } else {
    // forward
    http.Redirect(w, req, cloud.CLOUD_ADDR + "/api" + req.URL.Path, 301)
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
  api.commandBlock(veh, 176, w)
}

func (api *DroneAPI) handleSetHome(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {

  var lat, lon, alt float64
  var rel bool
  if postData["lat"] != nil {
    lat = postData["lat"].(float64)
  } else {
    lat = 0.0
  }

  if postData["lon"] != nil {
    lon = postData["lon"].(float64)
  } else {
    lat = 0.0
  }

  if postData["alt"] != nil {
    alt = postData["alt"].(float64)
  } else {
    alt = 0.0
  }

  if postData["relative"] != nil {
    rel = postData["relative"].(bool)
  } else {
    rel = false
  }

  veh.SetHome(float32(lat), float32(lon), float32(alt), rel)
  api.commandBlock(veh, 179, w)
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

func (api *DroneAPI) handleGetAllParams(veh *vehicle.Vehicle, w *http.ResponseWriter) {
  paramsRes := make(map[string]interface{})
  current, total, chunk := veh.GetAllParams()
  paramsRes["total"] = total
  paramsRes["current"] = current
  paramsRes["missing"] = veh.MissingParams()

  // JSON cannot encode NaNs
  for k, e := range chunk {
    if math.IsNaN(float64(e)) {
      chunk[k] = 0.0
    }
  }

  paramsRes["params"] = chunk
  api.SendAPIJSON(paramsRes, w)
}

func (api *DroneAPI) handleRefreshParams(veh *vehicle.Vehicle, w *http.ResponseWriter) {
  veh.RefreshParams()

  attempts := 0
  data := make(map[string]interface{})
  for {
    c, t, _ := veh.GetAllParams()
    if c >= t {
      data["Status"] = "OK"
      data["total"] = t
      api.SendAPIJSON(data, w)
      return
    }
    time.Sleep(50 * time.Millisecond)
    attempts++
    if attempts > 20 {
      break
    }
  }

  api.SendAPIError(fmt.Errorf("Failed to fetch all params."), w)
}

func (api *DroneAPI) handleGetSingleParam(veh *vehicle.Vehicle, name string, w *http.ResponseWriter) {
  var val float32
  var perr error
  if i, err := strconv.Atoi(name); err != nil {
    // look up by string
    val, perr = veh.GetParam(name)
  } else {
    val, perr = veh.GetParamByIndex(uint(i))
  }

  if perr != nil {
    api.SendAPIError(perr, w)
  } else {
    api.SendAPIJSON(val, w)
  }
}

func (api *DroneAPI) handleSetParam(veh *vehicle.Vehicle, path string, data map[string]interface{}, w *http.ResponseWriter) {
  val := data["value"].(float64)
  if err := veh.SetParam(path, float32(val)); err != nil {
    api.SendAPIError(err, w)
  } else {
    ret := make(map[string]interface{})
    ret["Status"] = "OK"
    api.SendAPIJSON(ret, w)
  }
}

func (api *DroneAPI) handleInput(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  t := postData["type"].(string)
  if t == "radio" {
    channels := [8]uint16{65535, 65535, 65535, 65535, 65535, 65535, 65535, 65535,}
    vals := postData["channels"].([]interface{})

    for i, e := range vals {
      arg := e.(float64)
      channels[i] = uint16(arg)
    }
    veh.SendRCOverride(channels)
    ret := make(map[string]interface{})
    ret["Status"] = "OK"
    api.SendAPIJSON(ret, w)
  } else {
    api.SendAPIError(fmt.Errorf("Invalid input type: " + t), w)
  }

}

func (api *DroneAPI) handleCommand(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  params := [7]float32{}
  cmd := 0.0

  if postData["command"] == nil {
    api.SendAPIError(fmt.Errorf("Command is required."), w)
    return
  } else {
    cmd = postData["command"].(float64)
  }

  if postData["args"] != nil {
    args := postData["args"].([]interface{})
    for i, e := range args {
      f := e.(float64)
      params[i] = float32(f)
    }
  }

  veh.DoGenericCommand(int(cmd), params)
  api.commandBlock(veh, int(cmd), w)
}

func (api *DroneAPI) handleTakeoff(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  params := [7]float32{}
  home := veh.GetHome()

  if postData["heading"] != nil {
    val := postData["heading"].(float64)
    params[3] = float32(val)
  }

  if postData["altitude"] != nil {
    val := postData["altitude"].(float64)
    params[6] = float32(val) + home["Altitude"]
  } else {
    params[6] = 10 + home["Altitude"]
  }

  if postData["lat"] != nil {
    val := postData["lat"].(float64)
    params[4] = float32(val)
  } else {
    params[4] = home["Latitude"]
  }

  if postData["long"] != nil {
    val := postData["long"].(float64)
    params[5] = float32(val)
  } else {
    params[5] = home["Longitude"]
  }

  veh.DoGenericCommand(mavlink.MAV_CMD_NAV_TAKEOFF, params)
  api.commandBlock(veh, mavlink.MAV_CMD_NAV_TAKEOFF, w)
}

func (api *DroneAPI) commandBlock(veh *vehicle.Vehicle, cmd int, w *http.ResponseWriter) {
  attempts := 0
  data := make(map[string]interface{})
  for {
    if attempts >= 6 {
      break
    }
    time.Sleep(250 * time.Millisecond)

    if op, ack := veh.GetLastSuccessfulCmd(); op == int(cmd) {
      // data["Status"] = "OK"
      data["Status"] = ack
      data["Command"] = cmd
      log.Println("Nulling last successful cmd")
      veh.NullLastSuccessfulCmd()
      api.SendAPIJSON(data, w)
      return
    } else {
      data["Status"] = ack
    }

    attempts++
  }

  // data["Status"] = "FAIL"
  data["Command"] = cmd
  api.SendAPIJSON(data, w)
}
