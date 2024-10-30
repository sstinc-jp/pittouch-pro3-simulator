package prooperate

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"os"
	"pro3sim/websql"
	"sync"
)

var conf struct {
	dbDir          string
	fileOperateDir string
}

func Setup(mux *mux.Router, dbDir string, fileOperateDir string) {
	conf.dbDir = dbDir
	conf.fileOperateDir = fileOperateDir

	_ = os.MkdirAll(fileOperateDir, 0755)

	mux.HandleFunc("/pjf/api/removeAllWebSQLDB", removeAllWebSQLDBHandler)
	// prooperate.jsのイベントを擬似的に発生させる機構
	mux.HandleFunc("/pjf/api/eventTrigger", eventTrigger)
	mux.HandleFunc("/pjf/api/eventNotification", eventNotification)

	// profileoperate
	mux.HandleFunc("/pjf/api/writeFile", writeFileHandler)
	mux.HandleFunc("/pjf/api/readFile", readFileHandler)
}

func removeAllWebSQLDBHandler(w http.ResponseWriter, r *http.Request) {
	websql.DeleteAllDatabases()
}

// addChannelされたchannel全てに、eventTriggerで受け取ったデータを配送する
func eventTrigger(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	for _, ch := range channels {
		select {
		case ch <- data:
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var channels = []chan []byte{}
var mutex sync.Mutex

func addChannel(ch chan []byte) {
	mutex.Lock()
	defer mutex.Unlock()
	channels = append(channels, ch)
}

func removeChannel(ch chan []byte) {
	mutex.Lock()
	defer mutex.Unlock()
	for i, c := range channels {
		if c == ch {
			channels = append(channels[:i], channels[i+1:]...)
			break
		}
	}
}

// websocketでeventを受け取る
func eventNotification(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 切断を検知するためのchannel
	disconnectedCh := make(chan struct{}, 1)
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
		disconnectedCh <- struct{}{}
	}()

	ch := make(chan []byte, 10)
	addChannel(ch)
	defer removeChannel(ch)

	for {
		select {
		case msg := <-ch:
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		case <-disconnectedCh:
			return
		}
	}

}
