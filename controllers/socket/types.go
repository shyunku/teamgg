package socket

import (
	socketio "github.com/googollee/go-socket.io"
	"sort"
	"strings"
	"sync"
	"team.gg-server/libs/db"
	"team.gg-server/models"
)

// types
const (
	EventTest = "test"

	EventJoinCustomConfigRoom        = "join_custom_config_room"
	EventCustomConfigOptimizeProcess = "custom_config/optimize_process"
	EventCustomConfigUpdated         = "custom_config/updated"
	EventCustomConfigViewers         = "custom_config/viewers"
)

type UserSocket struct {
	User *models.UserDAO
	Conn socketio.Conn
}

type Manager struct {
	mu                sync.RWMutex
	users             map[string]UserSocket
	sockets           map[string]UserSocket
	customConfigRooms map[string]map[string]struct{}
	Io                *socketio.Server
}

func NewSocketManager() *Manager {
	return &Manager{
		users:             make(map[string]UserSocket),
		sockets:           make(map[string]UserSocket),
		customConfigRooms: make(map[string]map[string]struct{}),
	}
}

func (sm *Manager) AddUser(user *models.UserDAO, conn socketio.Conn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	userSocket := UserSocket{
		Conn: conn,
	}
	if user != nil {
		userSocket.User = user
		sm.users[user.UserId] = userSocket
	}
	sm.sockets[conn.ID()] = userSocket
}

func (sm *Manager) RemoveUserByUserId(userId string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	userSocket, ok := sm.users[userId]
	if ok {
		delete(sm.sockets, userSocket.Conn.ID())
		delete(sm.users, userId)
	}
}

func (sm *Manager) RemoveUserByConnId(connId string) []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	leftConfigIds := make([]string, 0)
	userSocket, ok := sm.sockets[connId]
	if ok {
		delete(sm.sockets, connId)
		if userSocket.User != nil {
			delete(sm.users, userSocket.User.UserId)
		}
	}
	for configId, connections := range sm.customConfigRooms {
		if _, joined := connections[connId]; !joined {
			continue
		}
		delete(connections, connId)
		leftConfigIds = append(leftConfigIds, configId)
		if len(connections) == 0 {
			delete(sm.customConfigRooms, configId)
		}
	}
	return leftConfigIds
}

func (sm *Manager) GetUserByUserId(userId string) (UserSocket, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	userSocket, ok := sm.users[userId]
	return userSocket, ok
}

func (sm *Manager) TrackCustomConfigRoom(configId string, conn socketio.Conn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.customConfigRooms[configId] == nil {
		sm.customConfigRooms[configId] = make(map[string]struct{})
	}
	sm.customConfigRooms[configId][conn.ID()] = struct{}{}
}

func (sm *Manager) BroadcastCustomConfigViewers(configId string) {
	sm.mu.RLock()
	uids := make(map[string]struct{})
	for connId := range sm.customConfigRooms[configId] {
		if userSocket, exists := sm.sockets[connId]; exists && userSocket.User != nil {
			uids[userSocket.User.Uid] = struct{}{}
		}
	}
	sm.mu.RUnlock()

	puuidSet := make(map[string]struct{})
	for uid := range uids {
		identities, err := models.GetUserIdentityDAOs_byUid(db.Root, models.UserIdentityProviderRiot, uid)
		if err != nil {
			continue
		}
		for _, identity := range identities {
			if identity.ProviderSubject != "" {
				puuidSet[identity.ProviderSubject] = struct{}{}
			}
			separator := strings.LastIndex(identity.DisplayName, "#")
			if separator <= 0 || separator >= len(identity.DisplayName)-1 {
				continue
			}
			summoner, exists, err := models.GetSummonerDAO_byNameTag(
				db.Root, identity.DisplayName[:separator], identity.DisplayName[separator+1:],
			)
			if err == nil && exists {
				puuidSet[summoner.Puuid] = struct{}{}
			}
		}
	}

	puuids := make([]string, 0, len(puuidSet))
	for puuid := range puuidSet {
		puuids = append(puuids, puuid)
	}
	sort.Strings(puuids)
	sm.BroadcastToCustomConfigRoom(configId, EventCustomConfigViewers, map[string]interface{}{"puuids": puuids})
}

func (sm *Manager) BroadcastToCustomConfigRoom(configId string, event string, data interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	roomKey := RoomKey(configId)
	sm.Io.BroadcastToRoom("/", roomKey, event, data)
}

func (sm *Manager) MulticastToCustomConfigRoom(configId string, exceptUid string, event string, data interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	exceptSocket, ok := sm.GetUserByUserId(exceptUid)
	if !ok {
		//log.Debugf("configId: %s, exceptUid: %s, event: %s, data: %v", configId, exceptUid, event, data)
		sm.BroadcastToCustomConfigRoom(configId, event, data)
	} else {
		roomKey := RoomKey(configId)
		sm.Io.ForEach("/", roomKey, func(conn socketio.Conn) {
			if conn.ID() != exceptSocket.Conn.ID() {
				conn.Emit(event, data)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   *string     `json:"error"`
}

func NewResponse(success bool, data interface{}, err *string) Response {
	return Response{
		Success: success,
		Data:    data,
		Error:   err,
	}
}

func NewSuccess(data interface{}) Response {
	return NewResponse(true, data, nil)
}

func NewFailure(errMsg string) Response {
	return NewResponse(false, nil, &errMsg)
}

/* ---------------------- custom event data (must be minimized) ---------------------- */

const (
	TypeCustomConfigOptimizeProcessCombinating = "combinating"
	TypeCustomConfigOptimizeProcessCalculating = "calculating"
)

type CustomConfigOptimizeProcessData struct {
	Type     string  `json:"type"`
	Progress float64 `json:"progress"`
	Current  int64   `json:"current"`
	Total    int64   `json:"total"`
}
