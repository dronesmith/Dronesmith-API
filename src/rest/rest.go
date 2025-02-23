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


//
// The RESTful endpoint for everything in the API service.
//

package rest

import (
  "fmt"
  "logger"
  "net/http"
  "strconv"
  "runtime"
  "time"

  "cloud"
  "rest/apiservice"
)

const (
  LOC = "Las Vegas, USA"
  VER = "1.0.05"
)

var (
  GIT string
  initTime time.Time
)

func init() {
  initTime = time.Now()
}

type RestServer struct {
  addr string
  apiMux *http.ServeMux
  droneApi *apiservice.DroneAPI
}

func NewRestServer(addr string) *RestServer {
  return &RestServer{
    addr, nil, nil,
  }
}

func (r *RestServer) handleForward(w http.ResponseWriter, req *http.Request) {
  logger.Info("REQUEST", req.Method, req.URL.Path)
  http.Redirect(w, req, cloud.CLOUD_ADDR + "/api" + req.URL.Path, 302)
}

func (r *RestServer) whoami(w http.ResponseWriter, req *http.Request) {
  logger.Info("REQUEST", req.Method, req.URL.Path)
  fmt.Fprintf(w,
    "<html><head><title>%s</title></head><body><h1>%s</h1><p>%s</p></body></html>",
    "Dronesmith Technologies",
    "Hello, 世界.",
    "Dronesmith Core v" + VER + " (Git Hash " + GIT + ")" + "<br>" +
     runtime.Version() + " engine on a " + runtime.GOARCH + " " + runtime.GOOS + " system, with " +
     strconv.Itoa(runtime.NumGoroutine()) + " jobs running across " +
     strconv.Itoa(runtime.NumCPU()) +  " logical cores." +
      "<br>Proudly crafted with ♥ in "+LOC+"<br>System was last launched at "+initTime.String())
}

func (r *RestServer) Listen(dsAddr string) {
  r.apiMux = http.NewServeMux()
  r.droneApi = apiservice.NewDroneAPI(dsAddr, false, nil)

  r.apiMux.Handle(      "/drone/",    r.droneApi)
  // r.apiMux.Handle(      "/drones",    r.droneApi)
  r.apiMux.HandleFunc(  "/user/",     r.handleForward)
  r.apiMux.HandleFunc(  "/mission/",  r.handleForward)
  r.apiMux.HandleFunc(  "/",          r.rootHandler)
  r.apiMux.HandleFunc(  "/whoami",    r.whoami)

  logger.Info("API listening on", r.addr)
  logger.Error(http.ListenAndServe(r.addr, r.apiMux))
}

func (r *RestServer) rootHandler(w http.ResponseWriter, req* http.Request) {
  logger.Info("REQUEST", req.Method, req.URL.Path)
  logger.Info("Note: This is the default handler")

  if req.URL.Path != "/" {
    http.Error(w, http.StatusText(404), 404)
    return
  } else {
    fmt.Fprintf(w,
      "<html><head><title>%s</title></head><body><p>%s</p></body></html>",
      "DS API Service",
      "Dronesmith API Service is currently running, powered by many little gophers.")
  }
}
