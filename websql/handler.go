package websql

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sstinc-jp/go-sqlite3"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
)

var errUnmarshal = &SqlError{
	Code:    WEBSQL_UNKNOWN_ERR,
	Message: "internal error(invalid argument)",
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type OpenReq struct {
	Name                string      `json:"name"`
	Version             string      `json:"version"`
	DisplayName         string      `json:"displayName"`
	EstimatedSize       interface{} `json:"estimatedSize"`
	HasCreationCallback bool        `json:"hasCreationCallback"`
}

type OpenResp struct {
	DbId    uint32 `json:"dbId"`
	Created bool   `json:"created"`
}

func openHandler(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	var req OpenReq
	if err := json.Unmarshal(body, &req); err != nil {
		websqlLog.Warningf("unmarshal error %v", err)
		writeErrorResp(w, errUnmarshal)
		return
	}

	dbId, created, err := Open(req.Name, req.Version, req.HasCreationCallback)
	if err != nil {
		writeErrorResp(w, err)
		return
	}

	resp := OpenResp{
		DbId:    dbId,
		Created: created,
	}
	writeSuccessResp(w, &resp)
}

type TransactionReq struct {
	DbId uint32 `json:"dbId"`
}

type TransactionResp struct {
	TxId uint32 `json:"txId"`
}

type TransactionMsg struct {
	Cmd    string        `json:"cmd"`
	Stmt   string        `json:"statement"`
	Args   []interface{} `json:"args"`
	OldVer string        `json:"oldVersion"`
	NewVer string        `json:"newVersion"`
}

func transactionHandler(httpw http.ResponseWriter, r *http.Request) {

	websqlLog.Debugf(0x1, "transactionHandler start")
	defer websqlLog.Debugf(0x1, "transactionHandler end")

	conn, err := upgrader.Upgrade(httpw, r, nil)
	if err != nil {
		return
	}

	//remote := r.RemoteAddr
	// TODO 外部からのconnectはできなくする

	dbIdStr := r.URL.Query().Get("dbId")
	dbId, err := strconv.ParseUint(dbIdStr, 10, 32)
	if err != nil {
		websqlLog.Errorf("cannot parse dbId in query: dbIdStr=%v", dbIdStr)
		return
	}

	txId := uint32(0)
	for {
		var msg TransactionMsg

		err := conn.ReadJSON(&msg)
		if err != nil {
			if txId != 0 {
				websqlLog.Debugf(0x1, "websocket closed. calling abort for transaction")
				_ = Abort(txId)
			}
			return // 終了
		}

		websqlLog.Debugf(0x1, "transaction cmd=%v", msg.Cmd)
		switch msg.Cmd {
		case "begin":
			if txId != 0 {
				websqlLog.Errorf("beginTransaction before commit or abort")
				_ = Abort(txId)
				txId = 0
			}

			txId, err = BeginTransaction(uint32(dbId)) // XXX もはやuintである必要がない
			if err != nil {
				websqlLog.Debugf(0x1, "failed to begin transaction: %v", err)
				conn.WriteJSON(makeErrorResp(err))
				return
			}

			var resp BeginResp
			conn.WriteJSON(makeSuccessResp(&resp))

		case "exec":
			if txId == 0 {
				websqlLog.Errorf("exec called but tx is nil")
				conn.WriteJSON(makeErrorResp(errors.New("exec called but tx is nil")))
				continue
			}

			if msg.Stmt == "" {
				websqlLog.Debugf(0x1, "transaction statement missing")
				conn.WriteJSON(makeErrorResp(errUnmarshal))
				continue
			}
			lastInsertRowId, rowsAffected, rows, err := Exec(txId, msg.Stmt, msg.Args)
			if err != nil {
				websqlLog.Debugf(0x1, "exec failed: %v", err)
				conn.WriteJSON(makeErrorResp(err))
				break
			}

			resp := ExecResp{
				Rows:         rows,
				InsertId:     nil,
				RowsAffected: rowsAffected,
			}
			websqlLog.Debugf(0x1, "ExecResp=%v stmt=%v lastInsertRowId=%v rowsAffected=%v",
				resp, msg.Stmt, lastInsertRowId, rowsAffected)
			if lastInsertRowId >= 0 {
				resp.InsertId = &lastInsertRowId
			}
			conn.WriteJSON(makeSuccessResp(&resp))
			break

		case "commit":
			if txId == 0 {
				websqlLog.Errorf("commit called but tx is nil")
				conn.WriteJSON(makeErrorResp(errors.New("commit called but tx is nil")))
				continue
			}

			err = Commit(txId)
			txId = 0
			if err != nil {
				websqlLog.Debugf(0x1, "commit failed: %v", err)
				conn.WriteJSON(makeErrorResp(err))
				continue
			}

			var resp CommitResp
			conn.WriteJSON(makeSuccessResp(&resp))

			break

		case "abort":
			if txId == 0 {
				websqlLog.Errorf("abort called but tx is nil")
				conn.WriteJSON(makeErrorResp(errors.New("abort called but tx is nil")))
				continue
			}

			err = Abort(txId)
			txId = 0
			if err != nil {
				websqlLog.Debugf(0x1, "abort failed: %v", err)
				conn.WriteJSON(makeErrorResp(err))
				continue
			}
			var resp AbortResp
			conn.WriteJSON(makeSuccessResp(&resp))

			break

		case "changeVersion":
			if txId == 0 {
				websqlLog.Errorf("changeVersion called but tx is nil")
				conn.WriteJSON(makeErrorResp(errors.New("changeVersion called but tx is nil")))
				continue
			}

			err = ChangeDbVersion(txId, msg.OldVer, msg.NewVer)
			if err != nil {
				websqlLog.Debugf(
					0x1,
					"change version failed. old=%v new=%v: %v",
					msg.OldVer,
					msg.NewVer,
					err,
				)
				conn.WriteJSON(makeErrorResp(err))
				break
			}

			resp := ChangeVersionResp{}
			conn.WriteJSON(makeSuccessResp(&resp))
		default:
			websqlLog.Errorf("unknown command %v", msg.Cmd)
		}
	}
}

type ExecResp struct {
	Rows         []map[string]interface{} `json:"rows"`
	InsertId     *int64                   `json:"insertId,omitempty"`
	RowsAffected int64                    `json:"rowsAffected"`
}

type ChangeVersionResp struct {
}

type BeginResp struct {
}

type CommitResp struct {
}

type AbortResp struct {
}

type CloseReq struct {
	DbId uint32 `json:"dbId"`
}

type CloseResp struct {
}

func closeHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	var req CloseReq
	if err := json.Unmarshal(body, &req); err != nil {
		websqlLog.Warningf("unmarshal error %v", err)
		writeErrorResp(w, errUnmarshal)
		return
	}

	err = Close(req.DbId)
	if err != nil {
		writeErrorResp(w, err)
		return
	}

	var resp CloseResp
	writeSuccessResp(w, &resp)
}

type CloseAllResp struct {
}

// 現状は内部使用の非公開API
// WebKitNetworkProcessのcrash時にDBを強制closeするために使っている。
func closeAllHandler(w http.ResponseWriter, r *http.Request) {
	CloseAllConnections()

	var resp CloseAllResp
	writeSuccessResp(w, &resp)
}

type DbVersionReq struct {
	DbId uint32 `json:"dbId"`
}

type DbVersionResp struct {
	Version string `json:"version"`
}

func dbVersionHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	var req DbVersionReq
	if err := json.Unmarshal(body, &req); err != nil {
		websqlLog.Warningf("unmarshal error %v", err)
		writeErrorResp(w, errUnmarshal)
		return
	}

	ver, err := DatabaseVersion(req.DbId)
	if err != nil {
		writeErrorResp(w, err)
		return
	}

	resp := DbVersionResp{
		Version: ver,
	}
	writeSuccessResp(w, &resp)
}

func makeSuccessResp(data interface{}) map[string]interface{} {
	resp := map[string]interface{}{}
	resp["data"] = data
	return resp
}

func writeSuccessResp(w http.ResponseWriter, data interface{}) {
	resp := makeSuccessResp(data)
	body, _ := json.Marshal(resp)
	w.Write(body)
}

func makeErrorResp(err error) map[string]interface{} {
	resp := map[string]interface{}{}

	switch err := err.(type) {
	case *SqlError:
		resp["sqlerror"] = map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		}
	case *WebKitException:
		resp["exception"] = map[string]interface{}{
			"code":    err.Code,
			"name":    err.Name,
			"message": err.Message,
		}
	case sqlite3.Error: // 試した限り、*sqlite3.Errorではなくsqlite3.Errorで返ってくるようだ
		if err.Code == sqlite3.ErrFull {
			resp["sqlerror"] = map[string]interface{}{
				"code":    WEBSQL_QUOTA_ERR,
				"message": err.Error(),
			}
		} else {
			resp["error"] = map[string]interface{}{
				"name":    "UnknownError",
				"message": err.Error(),
			}
		}
	default:
		resp["error"] = map[string]interface{}{
			"name":    "UnknownError",
			"message": err.Error(),
		}
	}
	return resp
}

func writeErrorResp(w io.Writer, err error) {
	resp := makeErrorResp(err)
	body, _ := json.Marshal(resp)
	w.Write(body)
}

func Setup(mux *mux.Router, logger Logger) {
	setLogger(logger)

	mux.HandleFunc("/pjf/api/websql/open", openHandler)
	mux.HandleFunc("/pjf/api/websql/transaction", transactionHandler)
	//mux.HandleFunc("/pjf/api/websql/exec", execHandler)
	//mux.HandleFunc("/pjf/api/websql/commit", commitHandler)
	//mux.HandleFunc("/pjf/api/websql/abort", abortHandler)
	mux.HandleFunc("/pjf/api/websql/close", closeHandler)
	mux.HandleFunc("/pjf/api/websql/closeAll", closeAllHandler)
	mux.HandleFunc("/pjf/api/websql/dbversion", dbVersionHandler)
	//mux.HandleFunc("/pjf/api/websql/changeVersion", changeVersionHandler)
}
