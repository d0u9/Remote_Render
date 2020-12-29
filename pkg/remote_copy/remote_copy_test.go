package remote_copy

import (
    "time"
    "testing"
    log "github.com/sirupsen/logrus"
)

func init() {
    log.SetLevel(log.DebugLevel)
}

func TestRemoteCopyTo(t *testing.T) {
    rcp, _ := New("trident", "172.16.100.1", "/tmp/vspeed_render", true)
    go rcp.Watch()
    time.Sleep(time.Second * 1)
    rcp.Push("/tmp/pp/vvv")
    result := <- rcp.outC
    log.Info(result)
    rcp.Stop()
}
