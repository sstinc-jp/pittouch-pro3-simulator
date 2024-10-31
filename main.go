package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"path/filepath"
	"pro3sim/prooperate"
	"pro3sim/websql"
	"strings"
)

func main() {
	flagCtsDir := flag.String("ctsDir", "cts", "コンテンツセットのトップディレクトリ(index.htmlが存在するディレクトリ)。")
	flagPort := flag.Int("port", 8889, "サーバーのhttpポート番号。")
	flagPjfDir := flag.String("pjfDir", "pjf", "サーバーの/pjf/へのアクセス時に参照するディレクトリ。")
	flagDbDir := flag.String("dbDir", "db", "websqlのデータベースファイルを保存するディレクトリ。")
	flagProviderSetting := flag.String("providersetting", "providersetting.xml", "プロバイダ設定ファイルのパス")
	flagFileOperateDir := flag.String("fileOperateDir", "fileOperateDir", "ProFileOperateのAPIで読み書きするディレクトリ")
	flag.Parse()

	err := run(*flagCtsDir, *flagPjfDir, *flagPort, *flagProviderSetting, *flagDbDir, *flagFileOperateDir)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func run(ctsDir, pjfDir string, port int, providerPath string, dbDir string, fileOperateDir string) error {

	st, err := os.Stat(ctsDir)
	if err != nil {
		return fmt.Errorf("コンテンツセットのトップディレクトリをオープンできません: %v", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("コンテンツセットのトップディレクトリがディレクトリではありません: %v", ctsDir)
	}

	st, err = os.Stat(filepath.Join(pjfDir, "prooperate.js"))
	if err != nil {
		return fmt.Errorf("pjfディレクトリにprooperate.jsが存在しません: %v", err)
	}

	m := mux.NewRouter()
	websql.SetDBDir(dbDir)
	websql.Setup(m, nil)
	prooperate.Setup(m, dbDir, fileOperateDir)
	m.PathPrefix("/pjf/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		servePjfFile(w, r, pjfDir)
	})
	m.HandleFunc("/providersetting.xml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, providerPath)
	})
	m.PathPrefix("/").Handler(http.FileServer(http.Dir(ctsDir)))

	fmt.Printf("ポート%vでサーバーを開始します。ブラウザで http://localhost:%v/ にアクセスしてください。\n", port, port)

	err = http.ListenAndServe(fmt.Sprintf(":%v", port), m)
	if err != nil {
		return fmt.Errorf("httpサーバーを開始できません: %v", err)
	}
	return nil
}

func servePjfFile(w http.ResponseWriter, r *http.Request, pjfDir string) {
	path := strings.TrimPrefix(r.URL.Path, "/pjf/")
	path = filepath.Join(pjfDir, path)
	if strings.HasSuffix(path, ".js") {
		// 同名の_dev.jsが存在すればそれを優先してserveする。
		devName := path[:len(path)-3] + "_dev.js"
		st, _ := os.Stat(devName)
		if st != nil && !st.IsDir() {
			http.ServeFile(w, r, devName)
			return
		}
	}
	http.ServeFile(w, r, path)
}
