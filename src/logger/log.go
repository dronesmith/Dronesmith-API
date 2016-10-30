package logger

import (
  "os"
  "log"
  "time"
  "sync"
)

const (
  LOG_DIR = "logs/"
)

type droneLog struct {
  file    *os.File
  logger  *log.Logger
}

var (
  lock sync.Mutex
  logs map[string]*droneLog
  globalLogFile *os.File
  globalLogger *log.Logger
)

func init() {
  logs = make(map[string]*droneLog)
  log.Println(LOG_DIR + time.Now().Format(time.RFC3339) + ".log")
  globalLogFile, _ = os.Create(LOG_DIR + time.Now().Format(time.RFC3339) + ".log")
  globalLogger = log.New(globalLogFile, "[API] ", log.LstdFlags)
}

func LogDrone(name string, vals... interface{}) {
  lock.Lock()
  defer lock.Unlock()
  if dl, f := logs[name]; f {
    dl.logger.Println(vals...)
  } else {
    dl = &droneLog{}

    if file, err := os.Create(LOG_DIR + time.Now().Format(time.RFC3339) + ".log"); err != nil {
      Error("Failed to create log file for " + name + ". Reason:", err)
    } else {
      dl.file = file
      dl.logger = log.New(dl.file, "[DRONE-" + name + "]", log.LstdFlags)
      logs[name] = dl
    }
  }
}

func CloseLog(name string) {
  if dl, f := logs[name]; f {
    dl.file.Close()
    delete(logs, name)
  }
}

//
// Global log functions
//
func Debug(vals... interface{}) {
  log.SetPrefix("[DEBUG] ")
  log.Println(vals...)
}

func Warn(vals... interface{}) {
  globalLogger.SetPrefix("[WARN] ")
  globalLogger.Println(vals...)
  log.SetPrefix("[WARN] ")
  log.Println(vals...)
}

func Error(vals... interface{}) {
  globalLogger.SetPrefix("[ERROR] ")
  globalLogger.Println(vals...)
  log.SetPrefix("[ERROR] ")
  log.Println(vals...)
}

func Info(vals... interface{}) {
  globalLogger.SetPrefix("[INFO] ")
  globalLogger.Println(vals...)
  log.SetPrefix("[INFO] ")
  log.Println(vals...)
}
