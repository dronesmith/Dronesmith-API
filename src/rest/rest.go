
//
// The RESTful endpoint for everything in the API service.
//

package rest

import (
  "log"
  "net/http"
  "strconv"
)

const (

)

var (

)

type RestServer struct {
  port int
}

func NewRestServer(addr int) *RestServer {
  return &RestServer{
    addr,
  }
}

func (r *RestServer) Listen() {
  http.HandleFunc("/",  r.rootHandler)

  log.Fatal(http.ListenAndServe(":" + strconv.Itoa(r.port), nil))
}

func (r *RestServer) rootHandler(w http.ResponseWriter, req* http.Request) {
}
