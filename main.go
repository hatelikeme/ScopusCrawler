package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"./config"
	"./crawler"
	"./logger"
	"./storage"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

func main() {
	err := logger.Init()
	if err != nil {
		fmt.Println(err)
		return
	}
	conf, _ := config.ReadConfig("config.json")
	Storage := storage.MySqlStorage{
		DBType:   storage.MYSQL,
		User:     conf.Mysqluser,
		Password: conf.Mysqlpass,
		Address:  conf.Mysqladdress,
		DbName:   conf.Mysqldbname}
	err = Storage.Init()
	if err != nil {
		logger.Error.Println(err)
	}
	manager := crawler.Manager{}
	manager.Storage = Storage
	manager.Init("data-sources.json", conf.WorkersNumber)
	router := mux.NewRouter()
	router.HandleFunc("/request", RequestHandler(&manager))
	n := negroni.Classic()
	n.UseHandler(router)
	http.ListenAndServe(":9000", n)
}

func readRequest(request io.ReadCloser) (crawler.SearchRequest, error) {
	var req crawler.SearchRequest
	decoder := json.NewDecoder(request)
	err := decoder.Decode(&req)
	if err != nil {
		logger.Error.Println(err)
		return req, err
	}
	return req, nil
}

func RequestHandler(manager *crawler.Manager) http.HandlerFunc {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		searchRequest, err := readRequest(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
		}
		writer.WriteHeader(http.StatusOK)
		manager.StartCrawling(searchRequest)
	}
	return http.HandlerFunc(fn)
}
