package socket

import (
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"github.com/googollee/go-socket.io/engineio"
	"github.com/googollee/go-socket.io/engineio/transport"
	"github.com/googollee/go-socket.io/engineio/transport/websocket"
	log "github.com/shyunku-libraries/go-logger"
	"os"
	"team.gg-server/controllers/middlewares"
	"team.gg-server/libs/db"
	"team.gg-server/models"
)

var (
	SocketIO = NewSocketManager()
)

func UseSocket(r *gin.Engine) {
	g := r.Group("/socket.io")
	io := socketio.NewServer(&engineio.Options{
		Transports: []transport.Transport{
			//&polling.Transport{
			//	Client: &http.Client{
			//		Timeout: time.Minute,
			//	},
			//},
			&websocket.Transport{},
		},
	})
	SocketIO.Io = io
	userHandlers(io)

	log.Infof("socket.io server started")
	go func() {
		err := io.Serve()
		if err != nil {
			log.Error("socket.io server error: ", err)
			log.Fatal(err)
			os.Exit(1)
		}
	}()

	wrapper := func(c *gin.Context) {
		c.Request.Header.Del("Origin")
		io.ServeHTTP(c.Writer, c.Request)
	}

	g.Use(middlewares.UnsafeAuthMiddleware)
	g.GET("/*any", wrapper)
	g.POST("/*any", wrapper)
}

func userHandlers(io *socketio.Server) {
	io.OnConnect("/", func(s socketio.Conn) error {
		log.Infof("Socket connected: %v", s.ID())
		req := s.RemoteHeader()
		uid := req.Get("uid")
		log.Debugf("uid: %v", uid)

		if len(uid) != 0 {
			// get user
			userDAO, exists, err := models.GetUserDAO_byUid(db.Root, uid)
			if err != nil {
				log.Warn(err)
				return err
			}
			if !exists {
				log.Warnf("User not found: %v", uid)
				return nil
			}
			SocketIO.AddUser(userDAO, s)
		} else {
			SocketIO.AddUser(nil, s)
		}

		return nil
	})

	io.OnError("/", func(s socketio.Conn, e error) {
		log.Warnf("Socket error: %v", e)
	})

	io.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Debugf("Socket disconnected: %v", reason)
		configIds := SocketIO.RemoveUserByConnId(s.ID())
		s.LeaveAll()
		for _, configId := range configIds {
			SocketIO.BroadcastCustomConfigViewers(configId)
		}
	})

	io.OnEvent("/", EventTest, func(s socketio.Conn, msg string) {
		log.Debugf("Socket event: [%v] %v", s.ID(), msg)
		s.Emit(EventTest, msg)
	})

	/* ---------------------- custom ---------------------- */
	io.OnEvent("/", EventJoinCustomConfigRoom, func(s socketio.Conn, configId string) {
		log.Debugf("Socket event: [%v] %v", s.ID(), configId)
		s.Join(RoomKey(configId))
		SocketIO.TrackCustomConfigRoom(configId, s)
		SocketIO.BroadcastCustomConfigViewers(configId)
	})
}
