
//
// The RESTful endpoint for everything in the API service.
//

package rest

import (
  "fmt"
  "log"
  "net/http"
  "strconv"
  "runtime"
  "time"

  "cloud"
  "rest/apiservice"
)

const (
  LOC = "New York City, USA"
  VER = "1.0.0"
)

var (
  GIT string
  initTime time.Time
)

func init() {
  initTime = time.Now()
}

type RestServer struct {
  port int
  apiMux *http.ServeMux
  droneApi *apiservice.DroneAPI
}

func NewRestServer(addr int) *RestServer {
  return &RestServer{
    addr, nil, nil,
  }
}

func (r *RestServer) handleForward(w http.ResponseWriter, req *http.Request) {
  log.Println("REQUEST", req.Method, req.URL.Path)
  http.Redirect(w, req, cloud.CLOUD_ADDR + "/api" + req.URL.Path, 302)
}

func (r *RestServer) whoami(w http.ResponseWriter, req *http.Request) {
  log.Println("REQUEST", req.Method, req.URL.Path)
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

func (r *RestServer) Listen(port int) {
  r.apiMux = http.NewServeMux()
  r.droneApi = apiservice.NewDroneAPI(uint(port))

  r.apiMux.Handle(      "/drone/",    r.droneApi)
  r.apiMux.HandleFunc(  "/user/",     r.handleForward)
  r.apiMux.HandleFunc(  "/mission/",  r.handleForward)
  r.apiMux.HandleFunc(  "/",          r.rootHandler)
  r.apiMux.HandleFunc(  "/whoami",    r.whoami)

  log.Println("API listening on", r.port)
  log.Fatal(http.ListenAndServe(":" + strconv.Itoa(r.port), r.apiMux))
}

func (r *RestServer) rootHandler(w http.ResponseWriter, req* http.Request) {
  log.Println("REQUEST", req.Method, req.URL.Path)

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
