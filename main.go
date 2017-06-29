package main

import (
	"encoding/json"
	"fmt"
	"os"

	"./config"
	"./crawler"
	"./logger"
	"./storage"
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
	config, _ := config.ReadConfig("config.json")
	manager := crawler.Manager{}
	manager.Storage = Storage
	manager.Init("data-sources.json", config.WorkersNumber)
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

func readRequest(requestPath string) (crawler.SearchRequest, error) {
	var req crawler.SearchRequest
	file, err := os.Open(requestPath)
	if err != nil {
		logger.Error.Println("Unable to open request file.")
		return req, err
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&req)
	if err != nil {
		logger.Error.Println(err)
		return req, err
	}
	return req, nil
}
