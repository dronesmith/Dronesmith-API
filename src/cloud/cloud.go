package cloud

import (
  "bytes"
  "encoding/json"
  "fmt"
  "net/http"
  // "log"
)

const (
  CLOUD_ADDR = "http://localhost:4000"
)

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
  resp, err := http.Post(CLOUD_ADDR + "/rt/droneinfo", "application/json", bytes.NewBuffer(buf))

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

func RequestAPIGET(url, email, key string) (map[string]interface{}, error) {
  client := &http.Client{}
  if req, err := http.NewRequest("GET", CLOUD_ADDR + url, nil); err != nil {
    return nil, err
  } else {
    req.Header.Set("User-Email", email)
    req.Header.Set("User-Key", key)

    if res, err := client.Do(req); err != nil {
      return nil, err
    } else {
      switch res.StatusCode {
      case 200:
        decoder := json.NewDecoder(res.Body)
        t := make(map[string]interface{})
        err := decoder.Decode(&t)
        if err != nil {
          return nil, err
        }
        defer res.Body.Close()

        return t, nil
      default: // anything but 200
        decoder := json.NewDecoder(res.Body)
        t := map[string]string {
          "error": "",
        }
        err := decoder.Decode(&t)
        if err != nil {
            return nil, err
        }
        defer res.Body.Close()

        return nil, fmt.Errorf(t["error"])
      }
    }
  }
}
