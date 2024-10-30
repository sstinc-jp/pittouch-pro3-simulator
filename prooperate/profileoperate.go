package prooperate

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

type WriteRequest struct {
	FileName string `json:"fileName"`
	Data     string `json:"data"`
	IsAppend bool   `json:"isAppend"`
}

func writeFileHandler(w http.ResponseWriter, r *http.Request) {

	var req WriteRequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		return
	}
	var flag int
	if req.IsAppend {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	} else {
		flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	f, err := os.OpenFile(filepath.Join(conf.fileOperateDir, req.FileName), flag, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(req.Data)
}

type ReadRequest struct {
	FileName string `json:"fileName"`
}

func readFileHandler(w http.ResponseWriter, r *http.Request) {

	var req ReadRequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		return
	}

	f, err := os.Open(filepath.Join(conf.fileOperateDir, req.FileName))
	if err != nil {
		return
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}
	w.Write(data)
}
