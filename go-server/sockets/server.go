package sockets

import (
	"log"
	"net/http"
	"time"

	socketio "github.com/googollee/go-socket.io"
	"github.com/googollee/go-socket.io/engineio"
	"github.com/googollee/go-socket.io/engineio/transport"
	"github.com/googollee/go-socket.io/engineio/transport/polling"
	"github.com/googollee/go-socket.io/engineio/transport/websocket"
)

var Server *socketio.Server

func InitSocketIO() {
	server := socketio.NewServer(&engineio.Options{
		PingInterval: 30 * time.Second, // Long heartbeat interval
		PingTimeout:  60 * time.Second, // VERY long timeout for tunnel stability
		Transports: []transport.Transport{
			&polling.Transport{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			},
			&websocket.Transport{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			},
		},
	})
	// Some versions of go-socket.io might need extra config here, butnil is default.

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})

	server.OnEvent("/", "join", func(s socketio.Conn, room string) {
		s.Join(room)
		log.Printf("sid %s joined room %s", s.ID(), room)
	})

	// Add event handlers for updates
	server.OnEvent("/", "commonUpdate", func(s socketio.Conn, data interface{}) {
		server.BroadcastToRoom("/", "kitty-room", "commonUpdate", data)
	})

	server.OnEvent("/", "deviceUpdate", func(s socketio.Conn, data interface{}) {
		server.BroadcastToRoom("/", "kitty-room", "deviceUpdate", data)
	})

	server.OnEvent("/", "clientPing", func(s socketio.Conn, data map[string]interface{}) {
		// Just echo back with clientTime key that the frontend expects
		s.Emit("serverPong", map[string]interface{}{
			"clientTime": data["time"],
		})
	})

	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("socket error:", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("closed", reason)
	})

	Server = server
	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("socketio listen error: %v", err)
		}
	}()
}

func GetSocketHandler() http.Handler {
	return Server
}

// Helper to broadcast from ANY handler
func Broadcast(event string, data interface{}) {
	if Server != nil {
		Server.BroadcastToRoom("/", "kitty-room", event, data)
	}
}
