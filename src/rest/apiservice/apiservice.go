/**
 * Dronesmith API
 *
 * Authors
 *  Geoff Gardner <geoff@dronesmith.io>
 *
 * Copyright (C) 2016 Dronesmith Technologies Inc, all rights reserved.
 * Unauthorized copying of any source code or assets within this project, via
 * any medium is strictly prohibited.
 *
 * Proprietary and confidential.
 */

package apiservice

import (
  "logger"
  "fmt"
  "io"
  "math"
  "net/http"
  "regexp"
  "encoding/json"
  "time"
  "strconv"
  "strings"

  "mavlink/parser"
  "cloud"
  "dronemanager"
  "vehicle"
  CoreApi "vehicle/api"
)

type DroneAPI struct {
  addr string
  localMode bool
  manager *dronemanager.DroneManager
  localVehicle *vehicle.Vehicle
  idRgxp *regexp.Regexp
  nameRgxp *regexp.Regexp
  spltRgxp *regexp.Regexp
}

func NewDroneAPI(addr string, isLocal bool, writer io.Writer) *DroneAPI {
  api := &DroneAPI{}
  api.localMode = isLocal
  api.addr = addr

  if !api.localMode {
    api.manager = dronemanager.NewDroneManager(api.addr)
  } else {
    // create local vehicle
    api.localVehicle = vehicle.NewVehicle("local", writer)
  }

  api.idRgxp = regexp.MustCompile("[a-z0-9]{24}")
  api.nameRgxp = regexp.MustCompile("[A-Za-z0-9-]{5,24}")
  api.spltRgxp = regexp.MustCompile("/")

  if !api.localMode {
    go api.manager.Listen()
  } else {

  }

  return api
}

// func (api *DroneAPI) W3WForwardLookUp(w3w string) (lat float32, lon float32) {
//   return cloud.W3WForwardLookup(w3w)
// }

func (api *DroneAPI) Send404(w *http.ResponseWriter) {
  (*w).WriteHeader(http.StatusNotFound)
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
      logger.Warn("Request paniced!", r)
      fmt.Fprintf(w, "I couldn't parse your request. Make sure you are formating your JSON properly, including types.\n")
    }
  }()

  logger.Info("REQUEST", req.Method, req.URL.Path)

  paths := api.spltRgxp.Split(req.URL.Path, -1)
  email := req.Header.Get("User-Email")
  key := req.Header.Get("User-Key")

  var filteredPath []string
  for _, s := range paths {
    if s != "" {
      filteredPath = append(filteredPath, s)
    }
  }

  if filteredPath[0] == "drones" && req.Method == "GET" {
    jsonObj := api.manager.GetOnlineVehicles()
    api.SendAPIJSON(jsonObj, &w)
    return
  }

  if !api.localMode {

    if filteredPath[1] == "*" && req.Method == "GET" {
      jsonObj := api.manager.GetAllVehicleData()
      api.SendAPIJSON(jsonObj, &w)
      return
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
  } else {
    // Local mode only: send the status page back
    http.Redirect(w, req, "/index/status", 304)
  }

  // TODO match with name.
  if !api.idRgxp.MatchString(filteredPath[1]) && !api.nameRgxp.MatchString(filteredPath[1]) {
    api.Send404(&w)
    return
  }

  // Make sure user key and email are valid
  var droneData map[string]interface{}
  var veh *vehicle.Vehicle

  if !api.localMode {
    var found bool
    if found, droneData = api.Validate(email, key, filteredPath[1]); !found {
      api.Send403(&w)
      return
    }

    // Grab vehicle object for "live" data.
    veh = api.manager.FindVehicle(filteredPath[1])
  } else {
    veh = api.localVehicle
  }

  // If nil, vehicle isn't online.
  if veh == nil {
    droneData["online"] = false
    api.SendAPIError(fmt.Errorf("Drone not online."), &w)
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
    case "what3words": api.handleW3W(chunk, &w)
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
      } else {
        api.Send404(&w)
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

    toLowerJSON(pdata)

    switch filteredPath[2] {
    case "arm": api.handleArmDisarm(veh, true, &w)
    case "disarm": api.handleArmDisarm(veh, false, &w)
    case "ssh":
      if len(filteredPath) < 4 {
        api.Send404(&w)
      } else {
        if filteredPath[3] == "open" {
          api.handleTerminal(&w, filteredPath[1], true)
        } else if filteredPath[3] == "close" {
          api.handleTerminal(&w, filteredPath[1], false)
        }
      }
    case "takeoff": api.handleTakeoff(veh, pdata, &w)
    case "land": api.handleLand(veh, pdata, &w)
    case "goto": api.handleGuided(veh, pdata, &w)
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
    // 404 error
    api.Send404(&w)
  }
}

func (api *DroneAPI) handleArmDisarm(veh *vehicle.Vehicle, arming bool, w *http.ResponseWriter) {
  veh.SetModeAndArm(false, true, "", arming)
  api.commandBlock(veh, 176, w)
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

func (api *DroneAPI) handleTerminal(w *http.ResponseWriter, id string, enable bool) {
  if enable && api.manager.GetTerminal(id) != nil {
    api.SendAPIError(fmt.Errorf("SSH Proxy already open."), w)
    return
  } else if !enable && api.manager.GetTerminal(id) == nil {
    api.SendAPIError(fmt.Errorf("SSH Proxy already closed."), w)
    return
  }

  api.manager.UpdateTerminal(id, enable)

  attempts := 0
  for {
    if data := api.manager.GetTerminal(id); data != nil {
      if enable { // we requested terminal to be open, and it we got data.
        pData := make(map[string]interface{})
        pData["Status"] = "OK"
        pData["Info"] = data
        api.SendAPIJSON(pData, w)
        return
      }
    } else if data == nil {
      if !enable { // we requested the terminal to close, and it closed.
        pData := make(map[string]interface{})
        pData["Status"] = "OK"
        api.SendAPIJSON(pData, w)
        return
      }
    }
    time.Sleep(500 * time.Millisecond)
    attempts++
    if attempts > 5 {
      if enable {
        api.SendAPIError(fmt.Errorf("Could not create SSH Proxy."), w)
      } else {
        api.SendAPIError(fmt.Errorf("Could not close SSH Proxy."), w)
      }
      return
    }
  }

}

func (api *DroneAPI) handleW3W(data map[string]interface{}, w *http.ResponseWriter) {
  val, found := data["Position"]

  if found {
    // if this fails, there's something wrong with the core engine!
    pos := val.(CoreApi.Position)

    if conv, err := cloud.W3WGET(float64(pos.Latitude), float64(pos.Longitude)); err != nil {
      api.SendAPIError(err, w)
    } else {
      pos.What3words = conv
      api.SendAPIJSON(pos, w)
    }
  } else {
    api.SendAPIError(fmt.Errorf("Could get position data."), w)
  }
}

func (api *DroneAPI) handleSetHome(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  home := veh.GetHome()

  var lat, lon, alt float64
  var rel bool
  if postData["lat"] != nil {
    lat = postData["lat"].(float64)
  } else {
    lat = float64(home["Latitude"])
  }

  if postData["lon"] != nil {
    lon = postData["lon"].(float64)
  } else {
    lon = float64(home["Longitude"])
  }

  // if postData["What3words"] == {
  //   lat, lon = W3WForwardLookUp(postData["What3words"])
  // }

  if postData["alt"] != nil {
    alt = postData["alt"].(float64)
  } else {
    alt = float64(home["Altitude"])
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
  e := postData["enabled"].(bool)
  var ts float64

  if postData["timeout"] != nil {
    ts = postData["timeout"].(float64)
  }

  if t == "radio" {
    channels := [8]uint16{65535, 65535, 65535, 65535, 65535, 65535, 65535, 65535,}
    vals := postData["channels"].([]interface{})

    for i, e := range vals {
      arg := e.(float64)
      channels[i] = uint16(arg)
    }
    veh.SendRCOverride(channels, e, uint(ts))
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

  // if postData["What3words"] == {
  //   lat, lon = W3WForwardLookUp(postData["What3words"])
  // }

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

func (api *DroneAPI) handleLand(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  params := [7]float32{}
  // home := veh.GetHome()
  // loc := veh.GetGlobal()
  // useRelPos := false
  //
  // if postData["relativePos"] != nil {
  //   val := postData["relativePos"].(bool)
  //   useRelPos = val
  // }
  //
  // if postData["heading"] != nil {
  //   val := postData["heading"].(float64)
  //   params[3] = float32(val)
  // }
  //
  // if postData["lat"] != nil {
  //   val := postData["lat"].(float64)
  //
  //   if useRelPos {
  //     params[4] = loc["Latitude"] + float32(val)
  //   } else {
  //     params[4] = float32(val)
  //   }
  // } else {
  //   params[4] = loc["Latitude"]
  // }
  //
  // if postData["lon"] != nil {
  //   val := postData["lon"].(float64)
  //
  //   if useRelPos {
  //     params[5] = loc["Longitude"] + float32(val)
  //   } else {
  //     params[5] = float32(val)
  //   }
  // } else {
  //   params[5] = loc["Longitude"]
  // }
  //
  // params[6] = veh.GetMASLAlt()
  veh.DoGenericCommand(mavlink.MAV_CMD_NAV_LAND, params)
  api.commandBlock(veh, mavlink.MAV_CMD_NAV_LAND, w)
}

func (api *DroneAPI) handleGuided(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  params := [7]float32{}
  loc := veh.GetGlobal()
  useRelPos := false
  useRelAlt := true

  veh.SetModeAndArm(true, false, "Hold", true)

  if postData["relativealt"] != nil {
    val := postData["relativealt"].(bool)
    useRelAlt = val
  }

  if postData["relativepos"] != nil {
    val := postData["relativepos"].(bool)
    useRelPos = val
  }

  if postData["speed"] != nil {
    val := postData["speed"].(float64)
    params[0] = float32(val)
  } else {
    params[0] = -1
  }

  if postData["heading"] != nil {
    val := postData["heading"].(float64)
    params[3] = float32(val)
  }

  if postData["altitude"] != nil {
    val := postData["altitude"].(float64)

    if useRelAlt {
      params[6] = float32(val) + veh.GetMASLAlt()
    } else {
      params[6] = float32(val)
    }
  } else {
    params[6] = veh.GetMASLAlt()
  }

  if postData["lat"] != nil {
    val := postData["lat"].(float64)

    if useRelPos {
      params[4] = loc["Latitude"] + float32(val)
    } else {
      params[4] = float32(val)
    }
  } else {
    params[4] = loc["Latitude"]
  }

  if postData["lon"] != nil {
    val := postData["lon"].(float64)

    if useRelPos {
      params[5] = loc["Longitude"] + float32(val)
    } else {
      params[5] = float32(val)
    }
  } else {
    params[5] = loc["Longitude"]
  }

  veh.DoGenericCommand(mavlink.MAV_CMD_DO_REPOSITION, params)
  api.commandBlock(veh, mavlink.MAV_CMD_DO_REPOSITION, w)
}

func (api *DroneAPI) handleTakeoff(veh *vehicle.Vehicle, postData map[string]interface{}, w *http.ResponseWriter) {
  params := [7]float32{}
  global := veh.GetGlobal()
  useRelPos := false
  useRelAlt := true

  veh.SetModeAndArm(true, true, "Takeoff", true)

  if postData["relativepos"] != nil {
    val := postData["relativepos"].(bool)
    useRelPos = val
  }

  if postData["relativealt"] != nil {
    val := postData["relativealt"].(bool)
    useRelAlt = val
  }

  if postData["heading"] != nil {
    val := postData["heading"].(float64)
    params[3] = float32(val)
  }

  if postData["altitude"] != nil {
    val := postData["altitude"].(float64)

    if useRelAlt {
      params[6] = float32(val) + veh.GetMASLAlt()
    } else {
      params[6] = float32(val)
    }
  } else {
    params[6] = 10 + veh.GetMASLAlt()
  }

  if postData["lat"] != nil {
    val := postData["lat"].(float64)

    if useRelPos {
      params[4] = global["Latitude"] + float32(val)
    } else {
      params[4] = float32(val)
    }
  } else {
    params[4] = global["Latitude"]
  }

  if postData["lon"] != nil {
    val := postData["lon"].(float64)
    if useRelPos {
      params[5] = global["Longitude"] + float32(val)
    } else {
      params[5] = float32(val)
    }
  } else {
    params[5] = global["Longitude"]
  }

  veh.DoGenericCommand(mavlink.MAV_CMD_NAV_TAKEOFF, params)
  api.commandBlock(veh, mavlink.MAV_CMD_NAV_TAKEOFF, w)
}

func (api *DroneAPI) commandBlock(veh *vehicle.Vehicle, cmd int, w *http.ResponseWriter) {
  attempts := 0
  data := make(map[string]interface{})
  for {
    if attempts >= 10 {
      break
    }
    time.Sleep(250 * time.Millisecond)

    if op, ack, num := veh.GetLastSuccessfulCmd(); op == int(cmd) {
      // data["Status"] = "OK"
      data["Status"] = ack
      data["Command"] = cmd
      data["StatusCode"] = num
      if num != 10 {
        // logger.Debug("Nulling last successful cmd")
        veh.NullLastSuccessfulCmd()
        api.SendAPIJSON(data, w)
        return
      }
    } else {
      data["Status"] = ack
      data["StatusCode"] = num
    }

    attempts++
  }

  // data["Status"] = "FAIL"
  data["Command"] = cmd
  api.SendAPIJSON(data, w)
}

func toLowerJSON(data map[string]interface{}) {
  for str, _ := range data {
    nstr := strings.ToLower(str)
    data[nstr] = data[str]
  }
}
