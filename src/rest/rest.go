
//
// The RESTful endpoint for everything in the API service.
//

package rest

import (
  "fmt"
  "bytes"
  "log"
  "net/http"
  "strconv"
  "encoding/json"
)

const (
  CLOUD_ADDR = "localhost:4000"
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

//
// Static method. Asks the Cloud for drone information.
// Returns a map with the response data if good, error if bad response.
//
type UserDroneInfoRes struct {
  Status  string `json:"status"`
  User    map[string]interface{} `json:"user"`
  Drone   map[string]interface{} `json:"drone"`
}

func RequestDroneInfo(serial, user, pass string) (*UserDroneInfoRes, error) {
  postData := map[string]string {
    "serialId": serial,
    "email": user,
    "password": pass,
  }
  buf, _ := json.Marshal(postData)
  resp, err := http.Post("http://" + CLOUD_ADDR + "/rt/droneinfo", "application/json", bytes.NewBuffer(buf))

  if err != nil {
    return nil, err
  }

  switch resp.StatusCode {
  case 200:
    decoder := json.NewDecoder(resp.Body)
    t := UserDroneInfoRes{}
    err := decoder.Decode(&t)
    if err != nil {
      return nil, err
    }
    defer resp.Body.Close()

    return &t, nil
  default: // anything but 200
    decoder := json.NewDecoder(resp.Body)
    t := map[string]string {
      "error": "",
    }
    err := decoder.Decode(&t)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return nil, fmt.Errorf(t["error"])
  }

  return nil, err
}
