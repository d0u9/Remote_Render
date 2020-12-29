package app

import (
    "testing"
    log "github.com/sirupsen/logrus"
)

func TestApp(t *testing.T) {
    log.Info("TestApp...")
    app := New()
    app.Run()
}

