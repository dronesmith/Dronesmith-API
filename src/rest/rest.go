
//
// The RESTful endpoint for everything in the API service.
//

package rest

import (
  "fmt"
  "log"
  "net/http"
  "strconv"

  "cloud"
  "rest/apiservice"
)

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
  http.Redirect(w, req, cloud.CLOUD_ADDR + "/api" + req.URL.Path, 301)
}

func (r *RestServer) Listen() {
  r.apiMux = http.NewServeMux()
  r.droneApi = apiservice.NewDroneAPI(4002)

  r.apiMux.Handle(      "/drone/",    r.droneApi)
  r.apiMux.HandleFunc(  "/user/",     r.handleForward)
  r.apiMux.HandleFunc(  "/mission/",  r.handleForward)
  r.apiMux.HandleFunc(  "/",          r.rootHandler)

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
