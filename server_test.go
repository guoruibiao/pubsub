package pubsub

import "testing"

func TestServer_Run(t *testing.T) {
	server := NewServer()
	server.Run("localhost", 8080)
}
