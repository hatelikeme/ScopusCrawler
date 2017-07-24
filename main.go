package main

import (
	"encoding/json"
	"fmt"
	"./config"
	"./crawler"
	"./logger"
	"./storage"
	"os"
	"net/http"
	"github.com/gorilla/mux"
	"io"
)

func main() {
	err := logger.Init()
	if err != nil {
		fmt.Println(err)
		return
	}
	Storage := storage.MySqlStorage{DBType: storage.MYSQL, User: "root", Password: "temppwd", Address: "localhost:3306", DbName: "test"}
	err = Storage.Init()
	if err != nil {
		logger.Error.Println(err)
	}
	conf, _ := config.ReadConfig("config.json")
	manager := crawler.Manager{}
	manager.Storage = Storage
	manager.Init("data-sources.json", conf.WorkersNumber)

	router := mux.NewRouter()
	router.HandleFunc("/request", RequestHandler(&manager))
	router.HandleFunc("/")
	req, err := readRequest("request.json")
	if err != nil {
		logger.Error.Println(err)
		return
	}
	err = manager.StartCrawling(req)
	if err != nil {
		logger.Error.Println(err)
		return
	}
	fmt.Scanln()
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