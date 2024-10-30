"use strict";
// WebSQL互換API
// web APIと連携して、WebSQLの機能を提供する。
// https://www.w3.org/TR/webdatabase/
class DB {
    // DBをオープンする。
    constructor(arglen, name, version, displayName, estimatedSize, creationCallback) {
        this.pendingFuncs = [];
        this.running = false;
        // webkitは、openDatabaseのcreationCallback中のchangeVersionは全て失敗するので、
        // その挙動に合わせるためのフラグ。
        this.changeVersionForceFail = false;
        this.ws = null;
        // XXX webkitのWebSQLと違い、write操作(insert,updateなど)も成功する。
        this.readTransaction = this.transaction;
        // 引数チェック
        if (arglen < 4) {
            throw TypeError("Failed to execute 'openDatabase' on 'Window': 4 arguments required, but only " + arglen + " present.");
        }
        if (creationCallback != null && typeof creationCallback !== 'function') {
            throw new TypeError("Failed to execute 'openDatabase' on 'Window': The callback provided as parameter 5 is not a function.");
        }
        name = "" + name;
        version = "" + version;
        displayName = "" + displayName;
        let estimatedSizeStr = "" + estimatedSize;
        Object.defineProperty(this, "version", {
            get() {
                return this.getVersion(this.dbId);
            }
        });
        // 作成時にDBのあるなしチェックやversionチェックを行うので、sync呼び出し。
        let resp = this.accessSync("/pjf/api/websql/open", {
            name: name,
            version: version,
            displayName: displayName,
            estimatedSize: estimatedSizeStr,
            hasCreationCallback: creationCallback != null,
        });
        if (resp instanceof SqlError) {
            throw resp;
        }
        else if (resp instanceof Error) {
            throw resp;
        }
        else {
            this.dbId = resp["dbId"];
            this.ws = WsConn.connect(this.dbId);
            if (creationCallback != null && resp["created"] == true) {
                this.schedule(async () => {
                    this.changeVersionForceFail = true; // webkitの挙動に合わせるため、creationCallback中はchangeVersion呼び出しを失敗させる。
                    creationCallback(this);
                    this.changeVersionForceFail = false;
                });
            }
        }
    }
    // 実行中のタスクが無かったらlooperに積んで実行する。
    // もしタスク実行中ならqueueに入れる。
    schedule(f) {
        this.pendingFuncs.push(f);
        if (!this.running) {
            setTimeout(() => {
                this.doNext();
            }, 0);
        }
    }
    doNext() {
        if (this.running) {
            return;
        }
        let f = this.pendingFuncs.shift();
        if (f != null) {
            this.running = true;
            setTimeout(() => {
                if (f != null) {
                    let ret = f();
                    ret.finally(() => {
                        this.running = false;
                        this.doNext();
                    });
                }
                else {
                    this.running = false;
                    this.doNext();
                }
            }, 0);
        }
    }
    // versionプロパティを読んだ時に呼ばれる。webAPIはsync呼び出しであること。
    getVersion(dbId) {
        let resp = this.accessSync("/pjf/api/websql/dbversion", {
            dbId: dbId,
        });
        if (resp instanceof SqlError) {
            throw resp;
        }
        else if (resp instanceof Error) {
            throw resp;
        }
        else {
            return resp["version"];
        }
    }
    transaction(transactionCb, onError, onSuccess) {
        // 引数チェック
        if (arguments.length < 1) {
            throw TypeError("Failed to execute 'transaction' on 'Database': 1 argument required, but only 0 present.");
        }
        if (typeof transactionCb !== "function") {
            throw new TypeError("Failed to execute 'transaction' on 'Window': The callback provided as parameter 1 is not a function.");
        }
        if (onError != null && typeof onError !== 'function') {
            throw new TypeError("Failed to execute 'transaction' on 'Window': The callback provided as parameter 2 is not a function.");
        }
        if (onSuccess != null && typeof onSuccess !== 'function') {
            throw new TypeError("Failed to execute 'transaction' on 'Window': The callback provided as parameter 3 is not a function.");
        }
        this.transactionAsync(null, transactionCb, onError, onSuccess);
    }
    transactionAsync(changeVersion, transactionCb, onError, onSuccess) {
        this.schedule(async () => {
            let ws = null;
            try {
                ws = await this.ws;
                if (ws == null) {
                    throw new Error("cannot begin transaction");
                }
                await ws.begin();
                if (changeVersion != null) {
                    await ws.changeVersion(changeVersion[0], changeVersion[1]);
                }
                // ユーザーに渡すtrオブジェクト
                // db.transaction((tr) => {
                //    tr.executeSql(...)
                // のように使う。
                let queue = new Array();
                let tr = new Transaction(queue);
                // transactionCbの中で、tr.executeSql()が呼ばれる
                try {
                    // db.transaction((tr) => { の呼び出し
                    //    tr.executeSql(...)
                    if (transactionCb != null) {
                        transactionCb(tr);
                    }
                }
                catch (e) {
                    throw new Error("the SQLTransactionCallback was null or threw an exception");
                }
                // 呼ばれていたexecuteSqlを順に実行する
                for (let i = 0; i < queue.length; i++) {
                    let execSql = queue[i];
                    let resp;
                    try {
                        resp = await ws.exec(execSql.statement, execSql.args);
                    }
                    catch (e) { // onSuccess()のexceptionはここではcatchしない
                        if (execSql.onError != null) {
                            var shouldStop;
                            if (e instanceof SqlError) {
                                shouldStop = execSql.onError(tr, e);
                            }
                            else {
                                shouldStop = execSql.onError(tr, new SqlError(SqlError.UNKNOWN_ERR, ""));
                            }
                            if (shouldStop) {
                                throw new SqlError(SqlError.UNKNOWN_ERR, "the statement callback raised an exception or statement error callback did not return false");
                            }
                            else {
                                // 握りつぶして次の処理へ
                                continue;
                            }
                        }
                        else {
                            throw e;
                        }
                    }
                    if (execSql.onSuccess != null) {
                        execSql.onSuccess(tr, new ResultSet(resp["insertId"], resp["rowsAffected"], resp["rows"]));
                    }
                }
                await ws.commit();
                ws = null;
                try {
                    if (onSuccess != null) {
                        onSuccess();
                    }
                }
                catch (e) {
                }
            }
            catch (e) {
                if (onError != null) {
                    if (e instanceof SqlError) {
                        onError(e);
                    }
                    else {
                        onError(new SqlError(SqlError.UNKNOWN_ERR, e["message"]));
                    }
                }
            }
            finally {
                if (ws != null) {
                    await (ws === null || ws === void 0 ? void 0 : ws.abort());
                }
            }
        });
    }
    async close() {
        try {
            let trResp = await this.accessAsync("/pjf/api/websql/close", {
                dbId: this.dbId,
            });
            console.log("close OK");
        }
        catch (e) {
            console.log("close error " + e);
        }
    }
    // schema update用。versionが違ったら、cbを実行する。
    changeVersion(oldVersion, newVersion, cb, onError, onSuccess) {
        // 引数チェック
        if (arguments.length < 2) {
            throw TypeError("Failed to execute 'changeVersion' on 'Database': 2 argument required, but only " + arguments.length + " present.");
        }
        if (cb != null && typeof cb !== 'function') {
            throw new TypeError("Failed to execute 'changeVersion' on 'Database': The callback provided as parameter 2 is not a function.");
        }
        oldVersion = "" + oldVersion;
        newVersion = "" + newVersion;
        if (this.changeVersionForceFail) {
            this.schedule(async () => {
                if (onError != null) {
                    onError(new SqlError(SqlError.VERSION_ERR, "current version of the database and `oldVersion` argument do not match"));
                }
            });
            return;
        }
        this.transactionAsync([oldVersion, newVersion], cb, onError, onSuccess);
    }
    /**
     * sync呼び出しでserverにアクセスする
     * DBのopenやgetVersion()なと、sync呼び出ししなくてはならないAPIがあるため。
     */
    accessSync(path, reqBody) {
        let xhr = new XMLHttpRequest();
        xhr.open("POST", path, false);
        xhr.setRequestHeader("Content-Type", "application/json");
        //xhr.timeout = 1000; // int ms
        try {
            xhr.send(reqBody !== null ? JSON.stringify(reqBody) : null);
        }
        catch (e) {
            return new Error("AJAX failed " + e); // TODO exceptionにする？
        }
        if (xhr.status != 200) {
            // TODO
            return new Error("server down"); // TODO exceptionにする？
        }
        return DB.convertResponse(xhr.response);
    }
    /**
     * async呼び出しでserverにアクセスする
     */
    async accessAsync(path, reqBody) {
        let resp = await fetch(path, {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify(reqBody),
            keepalive: true // unload時に呼ばれても動くように
        });
        if (!resp.ok) {
            return new Error("server down"); // TODO exceptionにする？
        }
        return DB.convertResponse(await resp.text());
    }
    static convertResponse(response) {
        let resp;
        try {
            resp = JSON.parse(response);
        }
        catch (e) {
            return new Error("server invalid json"); // TODO exceptionにする？
        }
        if (resp["sqlerror"] != undefined) {
            return new SqlError(resp["sqlerror"]["code"], resp["sqlerror"]["message"]);
        }
        else if (resp["exception"] != undefined) {
            let error = Error();
            error.name = resp["exception"]["name"];
            error.message = resp["exception"]["message"];
            error["code"] = resp["exception"]["code"];
            console.log("received error from server. name=" + error.name + " msg=" + error.message);
            throw error;
        }
        else if (resp["error"] != undefined) {
            let error = Error();
            error.name = resp["error"]["name"];
            error.message = resp["error"]["message"];
            console.log("received error from server. name=" + error.name + " msg=" + error.message);
            throw error;
        }
        return resp["data"];
    }
}
// Transaction用のwebsocket connection
class WsConn {
    constructor(ws) {
        this.ws = ws;
    }
    static async connect(dbId) {
        return new Promise(function (resolve, reject) {
            const webSocket = new WebSocket(`ws://${location.host}/pjf/api/websql/transaction?dbId=` + dbId);
            webSocket.onopen = (event) => {
                resolve(new WsConn(webSocket));
            };
            webSocket.onmessage = (event) => {
                try {
                    let resp = DB.convertResponse(event.data);
                    if (resp instanceof SqlError || resp instanceof Error) {
                        reject(resp);
                    }
                    else {
                        resolve(resp);
                    }
                }
                catch (e) {
                    reject(e);
                }
            };
            webSocket.onerror = (event) => {
                reject(event);
            };
            webSocket.onclose = (event) => {
                reject(event);
            };
        });
    }
    async sendMessage(msg) {
        return new Promise((resolve, reject) => {
            if (this.ws.readyState != WebSocket.OPEN) {
                reject(new Error("ws already closed"));
            }
            this.ws.onmessage = (event) => {
                try {
                    let resp = DB.convertResponse(event.data);
                    if (resp instanceof SqlError || resp instanceof Error) {
                        reject(resp);
                    }
                    else {
                        resolve(resp);
                    }
                }
                catch (e) {
                    reject(e);
                }
            };
            this.ws.onerror = (event) => {
                reject(event);
            };
            this.ws.onclose = (event) => {
                reject(event);
            };
            this.ws.send(JSON.stringify(msg));
        });
    }
    async begin() {
        return this.sendMessage({
            "cmd": "begin",
        });
    }
    async changeVersion(oldVersion, newVersion) {
        return this.sendMessage({
            "cmd": "changeVersion",
            "oldVersion": oldVersion,
            "newVersion": newVersion,
        });
    }
    async exec(statement, args) {
        return this.sendMessage({
            "cmd": "exec",
            "statement": statement,
            "args": args,
        });
    }
    async commit() {
        return this.sendMessage({
            "cmd": "commit",
        });
    }
    async abort() {
        return this.sendMessage({
            "cmd": "abort",
        });
    }
    close() {
        this.ws.close();
    }
}
class ExecSql {
    constructor(statement, args, onSuccess, onError) {
        this.statement = statement;
        this.args = args;
        this.onSuccess = onSuccess;
        this.onError = onError;
    }
}
class Transaction {
    constructor(queue) {
        this.queue = queue;
    }
    executeSql(statement, args, onSuccess, onError) {
        // 引数チェックがあれば、先にexceptionを発生させる
        // 引数チェックに通れば、queueに積んでおく
        if (arguments.length < 1) {
            throw TypeError("Failed to execute 'executeSql' on 'SQLTransaction': 1 argument required, but only 0 present.");
        }
        if (args != null && !Array.isArray(args)) {
            // webkitではsequenceにconvertできないエラーをかえしていたが、arrayじゃないエラーでも良いだろう。
            //throw new TypeError("Failed to execute 'executeSql' on 'SQLTransaction': The provided value cannot be converted to a sequence.")
            throw new TypeError("Failed to execute 'executeSql' on 'SQLTransaction': The provided value is not an array.");
        }
        if (onSuccess != null && typeof onSuccess !== 'function') {
            throw new TypeError("Failed to execute 'executeSql' on 'SQLTransaction': The callback provided as parameter 3 is not a function.");
        }
        if (onError != null && typeof onError !== 'function') {
            throw new TypeError("Failed to execute 'executeSql' on 'SQLTransaction': The callback provided as parameter 4 is not a function.");
        }
        statement = "" + statement;
        this.queue.push(new ExecSql(statement, args, onSuccess, onError));
    }
}
class Rows extends Array {
    item(number) {
        if (number < 0 || number >= this.length) {
            throw new Error("out of range");
        }
        return this[number];
    }
}
class ResultSet {
    constructor(insertId, rowsAffected, rows) {
        this._insertId = insertId;
        this.rowsAffected = rowsAffected;
        this.rows = rows;
        Object.setPrototypeOf(this.rows, Rows.prototype);
        Object.defineProperty(this, "insertId", {
            get() {
                if (this._insertId == null) {
                    let err = new Error("Failed to read the 'insertId' property from 'SQLResultSet': The query didn't result in any rows being added.");
                    err.name = "InvalidAccessError";
                    throw err;
                }
                return this._insertId;
            }
        });
    }
}
class SqlError {
    constructor(code, message) {
        this.UNKNOWN_ERR = 0;
        this.DATABASE_ERR = 1;
        this.VERSION_ERR = 2;
        this.TOO_LARGE_ERR = 3;
        this.QUOTA_ERR = 4;
        this.SYNTAX_ERR = 5;
        this.CONSTRAINT_ERR = 6;
        this.TIMEOUT_ERR = 7;
        this.code = code;
        this.message = message;
    }
}
// The transaction failed for reasons unrelated to the database itself and not covered by any other error code.
SqlError.UNKNOWN_ERR = 0;
// The statement failed for database reasons not covered by any other error code.
SqlError.DATABASE_ERR = 1;
// The operation failed because the actual database version was not what it should be.
// For example, a statement found that the actual database version no longer matched the expected version of the Database or DatabaseSync object,
//  or the Database.changeVersion() or DatabaseSync.changeVersion() methods were passed a version that doesn't match the actual database version.
SqlError.VERSION_ERR = 2;
// The statement failed because the data returned from the database was too large.
// The SQL "LIMIT" modifier might be useful to reduce the size of the result set.
SqlError.TOO_LARGE_ERR = 3;
// The statement failed because there was not enough remaining storage space,
//  or the storage quota was reached and the user declined to give more space to the database.
SqlError.QUOTA_ERR = 4;
// The statement failed because of a syntax error,
//  or the number of arguments did not match the number of ? placeholders in the statement,
//  or the statement tried to use a statement that is not allowed, such as BEGIN, COMMIT,
//  or ROLLBACK, or the statement tried to use a verb that could modify the database but the transaction was read-only.
SqlError.SYNTAX_ERR = 5;
// An INSERT, UPDATE, or REPLACE statement failed due to a constraint failure.
// For example, because a row was being inserted and the value given for the primary key column duplicated the value of an existing row.
SqlError.CONSTRAINT_ERR = 6;
// A lock for the transaction could not be obtained in a reasonable time.
SqlError.TIMEOUT_ERR = 7;
// XXX openDatabaseSyncは提供しない。(pro2には無かったはずなので)
// - w3cでは、openDatabaseSyncはWebWorkerのcontextでしか定義されていない。
// - pro2のマニュアルには、「Worker 内で ProOperate などの JavaScript API 群や Web SQL Database を使用することはできません。」とある
// @ts-ignore
window.openDatabase = function (name, version, displayName, estimatedSize, creationCallback) {
    //console.log('replaced openDatabase called');
    try {
        var db = new DB(arguments.length, name, version, displayName, estimatedSize, creationCallback);
        window.addEventListener("unload", () => {
            db.close();
        });
        return db;
    }
    catch (e) {
        throw e;
    }
};
//# sourceMappingURL=websql.js.map