package websql

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/sstinc-jp/go-sqlite3"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const databaseInfoTable = "__pro_database_info"

const (
	WEBSQL_UNKNOWN_ERR    = 0
	WEBSQL_DATABASE_ERR   = 1
	WEBSQL_VERSION_ERR    = 2
	WEBSQL_TOO_LARGE_ERR  = 3
	WEBSQL_QUOTA_ERR      = 4
	WEBSQL_SYNTAX_ERR     = 5
	WEBSQL_CONSTRAINT_ERR = 6
	WEBSQL_TIMEOUT_ERR    = 7
)

const (
	WEBKIT_INDEX_SIZE_ERR              = 1
	WEBKIT_DOMSTRING_SIZE_ERR          = 2
	WEBKIT_HIERARCHY_REQUEST_ERR       = 3
	WEBKIT_WRONG_DOCUMENT_ERR          = 4
	WEBKIT_INVALID_CHARACTER_ERR       = 5
	WEBKIT_NO_DATA_ALLOWED_ERR         = 6
	WEBKIT_NO_MODIFICATION_ALLOWED_ERR = 7
	WEBKIT_NOT_FOUND_ERR               = 8
	WEBKIT_NOT_SUPPORTED_ERR           = 9
	WEBKIT_INUSE_ATTRIBUTE_ERR         = 10
	WEBKIT_INVALID_STATE_ERR           = 11
	WEBKIT_SYNTAX_ERR                  = 12
	WEBKIT_INVALID_MODIFICATION_ERR    = 13
	WEBKIT_NAMESPACE_ERR               = 14
	WEBKIT_INVALID_ACCESS_ERR          = 15
	WEBKIT_VALIDATION_ERR              = 16
	WEBKIT_TYPE_MISMATCH_ERR           = 17
	WEBKIT_SECURITY_ERR                = 18
	WEBKIT_NETWORK_ERR                 = 19
	WEBKIT_ABORT_ERR                   = 20
	WEBKIT_URL_MISMATCH_ERR            = 21
	WEBKIT_QUOTA_EXCEEDED_ERR          = 22
	WEBKIT_TIMEOUT_ERR                 = 23
	WEBKIT_INVALID_NODE_TYPE_ERR       = 24
	WEBKIT_DATA_CLONE_ERR              = 25
)

var databases = map[uint32]*sql.DB{}
var transactions = map[uint32]*TxWrapper{}
var nextId uint32
var lock sync.Mutex

type TxWrapper struct {
	tx                 *sql.Tx
	db                 *sql.DB
	isLastActionInsert bool
}

// WebSQL仕様の、SQLError型に相当
// https://www.w3.org/TR/webdatabase/#errors-and-exceptions
type SqlError struct {
	Code    int
	Message string
	Err     error
}

func (e *SqlError) Error() string {
	return fmt.Sprintf("SqlError Code=%v Message=%v Err=%v", e.Code, e.Message, e.Err)
}

// 内部エラー
type WebKitException struct {
	Code    int
	Name    string
	Message string
	Err     error
}

func (e *WebKitException) Error() string {
	return fmt.Sprintf("WebKitException Code=%v Message=%v Err=%v", e.Code, e.Message, e.Err)
}

var dbDir = ""

// dbの保存ディレクトリを変更する。""ならカレントディレクトリが使われる。
func SetDBDir(dirName string) {
	dbDir = dirName
	if dirName != "" {
		os.MkdirAll(dirName, 0755)
	}
}

var beginHook func(db *sql.DB, log Logger) error

// transactionのbegin時に呼び出すhookを登録する
func SetBeginHook(hook func(db *sql.DB, log Logger) error) {
	beginHook = hook
}

// databaseをopenする。
// databaseId, created, errorを返す。
func Open(name string, version string, hasCreationCallback bool) (uint32, bool, error) {

	websqlLog.Debugf(0x1, "Open. name=%v, ver=%v", name, version)

	// '$', '&', '+', ',', '/', ':', ';', '=', '?', '@' あたりをescapeしてくれる。
	// '/'以外はescapeしなくても良いのだが、ファイルに記号が入るのは何となく気持ち悪いので。
	name = url.QueryEscape(name) + "_" + hex.EncodeToString([]byte(name)) + ".db"
	if dbDir != "" {
		name = filepath.Join(dbDir, name)
	}

	db, err := sql.Open("sqlite3", name)
	if err != nil {
		websqlLog.Debugf(0x1, "Open failed: %v", err)
		return 0, false, err
	}

	exists, err := doesDatabaseInfoExists(db)
	websqlLog.Debugf(0x1, "exists=%v, Err=%v\n", exists, err)
	if err != nil {
		websqlLog.Warningf("cannot get databaseinfo: %v", err)
		db.Close()
		return 0, false, err
	}

	if exists {
		// 既に同名のDBがあるなら、今のversionと同じかチェック。versionが違ったらINVALID_STATE_ERRをthrowする。
		websqlLog.Debugf(0x1, "DB exists")
		if version != "" {
			curVer, err := getDatabaseVersion(db)
			websqlLog.Debugf(0x1, "db ver=%v, %v", curVer, err)
			if err != nil {
				websqlLog.Errorf("getDatabaseVersion() failed. err=%v", err)
				db.Close()
				return 0, false, err
			}
			if curVer != version {
				db.Close()
				websqlLog.Debugf(0x1, "DB version mismatch. curVer=%v, ver=%v", curVer, version)
				return 0, false, &WebKitException{
					Code:    WEBKIT_INVALID_STATE_ERR,
					Name:    "InvalidStateError",
					Message: fmt.Sprintf("Failed to execute 'openDatabase' on 'Window': unable to open database, version mismatch, '%v' does not match the currentVersion of '%v'", version, curVer),
					Err:     err,
				}
			}
		}
	} else {
		websqlLog.Debugf(0x1, "DB not exist")
		websqlLog.Debugf(0x1, "setting auto_vacuum=full")
		_, err := db.Exec(fmt.Sprintf("PRAGMA auto_vacuum = full"))
		if err != nil {
			websqlLog.Warningf("cannot set auto_vacuum=full. error=%v", err)
		}

		websqlLog.Debugf(0x1, "creating info")
		err = createDatabaseInfo(db)
		if err != nil {
			websqlLog.Warningf("cannot create DB info. error=%v", err)
			db.Close()
			return 0, false, err
		}

		// creationCallbackがあれば、versionは""にする (なぜ？？？)
		// creationCallbackが無ければ、versionは引数で渡されたものを使う。
		var ver string
		if hasCreationCallback {
			ver = ""
		} else {
			ver = version
		}
		err = setDatabaseVersion(db, ver)
		if err != nil {
			websqlLog.Warningf("cannot set DB version. err=%v", err)
			db.Close()
			return 0, false, err
		}
	}

	dbId := atomic.AddUint32(&nextId, 1)
	lock.Lock()
	databases[dbId] = db
	lock.Unlock()

	return dbId, !exists, nil
}

func DatabaseVersion(dbId uint32) (string, error) {
	lock.Lock()
	db := databases[dbId]
	lock.Unlock()
	if db == nil {
		return "", &SqlError{
			Code:    WEBSQL_UNKNOWN_ERR,
			Message: "internal error(db not found)",
			Err:     nil,
		}
	}

	rows, err := db.Query(fmt.Sprintf("SELECT * from %v", databaseInfoTable))
	if err != nil {
		return "", err
	}
	infos, err := buildStruct(rows)
	if err != nil {
		return "", err
	}
	if len(infos) < 1 {
		return "", nil
	}
	verStr, ok := infos[0]["version"].(string)
	if !ok {
		return "", errors.New("version is not string")
	}
	return verStr, nil
}

func BeginTransaction(dbId uint32) (uint32, error) {

	lock.Lock()
	db := databases[dbId]
	lock.Unlock()
	if db == nil {
		return 0, &SqlError{
			Code:    WEBSQL_UNKNOWN_ERR,
			Message: "internal error(db not found)",
			Err:     nil,
		}
	}

	if beginHook != nil {
		err := beginHook(db, websqlLog)
		if err != nil {
			websqlLog.Errorf("cannot set max_page_count: %v", err)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		websqlLog.Debugf(0x1, "db.begin error: %v", err)
		return 0, &SqlError{
			Code:    WEBSQL_UNKNOWN_ERR,
			Message: err.Error(),
			Err:     nil,
		}
	}

	txWrapper := TxWrapper{tx: tx, db: db, isLastActionInsert: false}
	// Exec()内で、INSERTかそうじゃないかを判断するためにhookを登録する
	// この値は、Exec()の最初にfalseにされる。
	conn := getConn(tx)
	conn.RegisterAuthorizer(func(action int, arg1, arg2, arg3 string) int {
		if action == sqlite3.SQLITE_INSERT {
			lock.Lock()
			txWrapper.isLastActionInsert = true
			lock.Unlock()
		}
		websqlLog.Debugf(0x1, "lastAction=%v", action)
		return sqlite3.SQLITE_OK
	})

	txId := atomic.AddUint32(&nextId, 1)
	lock.Lock()
	transactions[txId] = &txWrapper
	lock.Unlock()

	// commitが呼ばれないといつまでもtransactionが残ってしまうので、いつまでも呼ばれなかったらtransactionをabortする。
	time.AfterFunc(time.Minute*5, func() {
		lock.Lock()
		_, ok := transactions[txId]
		delete(transactions, txId)
		lock.Unlock()

		// commit中にRollback()を呼ばない事を保証するため、以下のようにする。
		// - commitHandlerでは、先にmapから抜いてからCommit()する。
		// - timeoutした場合、mapに存在していた時だけRollback()する。
		if ok {
			_ = tx.Rollback()
		}
	})

	return txId, nil
}

func getConn(tx *sql.Tx) *sqlite3.SQLiteConn {
	a := reflect.ValueOf(tx).
		Elem().
		FieldByName("dc"). // *sql.driverConn
		Elem().            // sql.driverConn
		FieldByName("ci"). // driver.Conn (interface)
		Elem()             // *sqlite3.SQLiteConn

	conn := (*sqlite3.SQLiteConn)(unsafe.Pointer(a.Pointer()))
	return conn
}

// lastInsertRowId, rowsAffected, rowsデータ, を返す
// lastInsertRowIdは、INSERT以外の時は-1を返す。
func Exec(txId uint32, statement string, args []interface{}) (int64, int64, []map[string]interface{}, error) {

	lock.Lock()
	tx := transactions[txId]
	lock.Unlock()
	if tx == nil {
		return 0, 0, nil, &SqlError{
			Code:    WEBSQL_UNKNOWN_ERR,
			Message: "tx missing (aborted?)",
		}
	}
	tx.isLastActionInsert = false

	conn := getConn(tx.tx)
	_, totalChanges1 := conn.GetInfo()

	rows, err := tx.tx.Query(statement, args...)
	if err != nil {
		websqlLog.Debugf(0x1, "tx.Query error: %v", err)
		return 0, 0, nil, &SqlError{
			Code:    WEBSQL_SYNTAX_ERR,
			Message: err.Error(),
			Err:     err,
		}
	}

	data, err := buildStruct(rows)
	if err != nil {
		websqlLog.Debugf(0x1, "buildStruct error: %v %T", err, err)
		return 0, 0, nil, err
	}

	lastInsertRowId, totalChanges2 := conn.GetInfo()
	lock.Lock()
	if !tx.isLastActionInsert {
		lastInsertRowId = -1
	}
	lock.Unlock()

	websqlLog.Debugf(0x1, "totalChanges1=%v, totalChanges2=%v", totalChanges1, totalChanges2)

	return lastInsertRowId, totalChanges2 - totalChanges1, data, nil
}

func ChangeDbVersion(txId uint32, oldVer string, newVer string) error {

	lock.Lock()
	tx := transactions[txId]
	lock.Unlock()
	if tx == nil {
		return &SqlError{
			Code:    WEBSQL_UNKNOWN_ERR,
			Message: "tx missing (aborted?)",
		}
	}

	ver, err := getDatabaseVersionByTx(tx.tx)
	if err != nil {
		return &SqlError{
			Code:    WEBSQL_UNKNOWN_ERR,
			Message: "tx missing (aborted?)",
		}
	}
	if ver != oldVer {
		return &SqlError{
			Code:    WEBSQL_VERSION_ERR,
			Message: "current version of the database and `oldVersion` argument do not match",
		}
	}

	if err := setDatabaseVersionByTx(tx.tx, newVer); err != nil {
		return &SqlError{
			Code:    WEBSQL_UNKNOWN_ERR,
			Message: "tx missing (aborted?)",
		}
	}

	return nil
}

func Commit(txId uint32) error {
	lock.Lock()
	tx := transactions[txId]
	delete(transactions, txId)
	lock.Unlock()
	if tx == nil {
		return &SqlError{
			Code:    WEBSQL_DATABASE_ERR,
			Message: "tx missing (aborted?)",
		}
	}

	err := tx.tx.Commit()
	if err != nil {
		websqlLog.Debugf(0x1, "tx.commit error: %v", err)
		_ = tx.tx.Rollback() // commitの失敗はどうしようもないので、rollbackする
		return &SqlError{
			Code:    WEBSQL_DATABASE_ERR,
			Message: err.Error(),
			Err:     err,
		}
	}

	return nil
}

func Abort(txId uint32) error {
	lock.Lock()
	tx := transactions[txId]
	delete(transactions, txId)
	lock.Unlock()
	if tx == nil {
		return &SqlError{
			Code:    WEBSQL_DATABASE_ERR,
			Message: "tx missing (aborted?)",
		}
	}

	err := tx.tx.Rollback()
	if err != nil {
		websqlLog.Debugf(0x1, "tx.rollback error: %v", err)
		return &SqlError{
			Code:    WEBSQL_DATABASE_ERR,
			Message: err.Error(),
			Err:     err,
		}
	}

	return nil
}

func Close(dbId uint32) error {
	lock.Lock()
	db := databases[dbId]
	delete(databases, dbId)
	lock.Unlock()

	websqlLog.Debugf(0x1, "Close dbId=%v", dbId)

	if db == nil {
		return &SqlError{
			Code:    WEBSQL_DATABASE_ERR,
			Message: "db missing (already closed?)",
		}
	}

	db.Close()
	return nil
}

func CloseAllConnections() {
	websqlLog.NoticeEventf("CloseAllConnections")
	lock.Lock()
	defer lock.Unlock()

	closeConnectionsLocked()
}

func closeConnectionsLocked() {
	for _, tx := range transactions {
		tx.tx.Rollback()
	}
	transactions = map[uint32]*TxWrapper{}
	for _, db := range databases {
		db.Close()
	}
	databases = map[uint32]*sql.DB{}
}

func DeleteAllDatabases() {
	lock.Lock()
	defer lock.Unlock()

	closeConnectionsLocked()

	files, err := ioutil.ReadDir(dbDir)
	if err != nil {
		return
	}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".db") {
			path := filepath.Join(dbDir, f.Name())
			os.Remove(path)
			websqlLog.Debugf(0x1, "DeleteAllDatabases(). file=%v", path)
		}
	}
}

func createDatabaseInfo(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf(`
CREATE TABLE %v (
	id INTEGER PRIMARY KEY,
	version TEXT default ""
)
`, databaseInfoTable))
	return err
}

func doesDatabaseInfoExists(db *sql.DB) (bool, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type=? AND name=?", []interface{}{"table", databaseInfoTable}...)
	if err != nil {
		return false, err
	}
	data, err := buildStruct(rows)
	if err != nil {
		return false, err
	}
	if len(data) == 1 {
		return true, nil
	} else {
		return false, nil
	}
}

func setDatabaseVersion(db *sql.DB, version string) error {
	_, err := db.Exec(fmt.Sprintf("REPLACE INTO %v ( id, version ) VALUES ( 0, ? );", databaseInfoTable), []interface{}{version}...)
	return err
}

func setDatabaseVersionByTx(tx *sql.Tx, version string) error {
	_, err := tx.Exec(fmt.Sprintf("REPLACE INTO %v ( id, version ) VALUES ( 0, ? );", databaseInfoTable), []interface{}{version}...)
	return err
}

func getDatabaseVersion(db *sql.DB) (string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT * from %v", databaseInfoTable))
	if err != nil {
		return "", err
	}
	infos, err := buildStruct(rows)
	if err != nil {
		return "", err
	}
	if len(infos) < 1 {
		return "", nil
	}
	verStr, ok := infos[0]["version"].(string)
	if !ok {
		return "", errors.New("version is not string")
	}
	return verStr, nil
}

func getDatabaseVersionByTx(tx *sql.Tx) (string, error) {
	rows, err := tx.Query(fmt.Sprintf("SELECT * from %v", databaseInfoTable))
	if err != nil {
		return "", err
	}
	infos, err := buildStruct(rows)
	if err != nil {
		return "", err
	}
	if len(infos) < 1 {
		return "", nil
	}
	verStr, ok := infos[0]["version"].(string)
	if !ok {
		return "", errors.New("version is not string")
	}
	return verStr, nil
}

func buildStruct(rows *sql.Rows) ([]map[string]interface{}, error) {

	ret := make([]map[string]interface{}, 0)

	cols, err := rows.Columns()
	if err != nil {
		websqlLog.Debugf(0x1, "cannot get cols %v", err)
		return nil, err
	}
	for rows.Next() {
		vers := make([]interface{}, len(cols))
		versp := make([]interface{}, len(cols))
		for i := 0; i < len(cols); i++ {
			versp[i] = &vers[i]
		}
		rows.Scan(versp[:]...)

		row := map[string]interface{}{}
		for i := 0; i < len(cols); i++ {
			row[cols[i]] = vers[i]
		}

		ret = append(ret, row)
	}
	return ret, rows.Err()
}
