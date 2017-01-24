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

package cloud

import (
  "bytes"
  "encoding/json"
  "fmt"
  "net/http"
  "strconv"
  // "logger"
)

const (
  W3W_API_KEY = "BXIP296D"
)

var (
  CLOUD_ADDR string
)

func InitCloud(addr string) {
  CLOUD_ADDR = addr
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

func RequestDroneInfo(serial, simId, user, pass string) (*UserDroneInfoRes, error) {
  postData := map[string]string {
    "serialId": serial,
    "simId": simId,
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

func W3WGET(lat float64, lon float64) (string, error) {
  client := &http.Client{}

  latStr := strconv.FormatFloat(lat, 'E', -1, 64)
  lonStr := strconv.FormatFloat(lon, 'E', -1, 64)

  if req, err := http.NewRequest("GET", "https://api.what3words.com/v2/reverse?coords=" +
    latStr+","+lonStr+"&key="+W3W_API_KEY +
    "&lang=en&format=json&display=full", nil); err != nil {
    return "", err
  } else {

    if res, err := client.Do(req); err != nil {
      return "", err
    } else {
      switch res.StatusCode {
      case 200:
        decoder := json.NewDecoder(res.Body)
        t := make(map[string]interface{})
        err := decoder.Decode(&t)
        if err != nil {
          return "", err
        }
        defer res.Body.Close()

        wordsStr := t["words"].(string)

        return wordsStr, nil
      default: // anything but 200
        decoder := json.NewDecoder(res.Body)
        t := map[string]string {
          "error": "",
        }
        err := decoder.Decode(&t)
        if err != nil {
            return "", err
        }
        defer res.Body.Close()

        return "", fmt.Errorf(t["error"])
      }
    }
  }
}
