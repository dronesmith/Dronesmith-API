package apiservice

import (
  "fmt"
  "net/http"
  "dronemanager"
)

type APIService struct {
  port uint
  manager *dronemanager.DroneManager
}

func NewAPIService(port uint) *APIService {
  api := &APIService{}
  api.port = port
  api.manager = dronemanager.NewDroneManager(api.port)

  go api.manager.Listen()

  return api
}

func (api *APIService) ServeHTTP(w http.ResponseWriter, req *http.Request) {
  // log.Println("REQUEST", req.Method, req.URL.Path)

    fmt.Fprintf(w,
      "<html><head><title>%s</title></head><body><p>%s</p></body></html>",
      "DS API Service",
      "API Request.")
}
