package crawler

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"../query"
	"../config"
	"../storage"
	"github.com/tidwall/gjson"
	"log"
)

type Manager struct {
	DataSources []DataSource
	Queue       chan SearchRequest
	WorkerQueue chan chan SearchRequest
	Storage     storage.MySqlStorage
}

func (manager *Manager) Init(dataSourcesPath string, workersNumber int) error {
	ds, err := manager.readDataSources(dataSourcesPath)
	if err != nil {
		return err
	}
	manager.DataSources = ds
	manager.Queue = make(chan SearchRequest, 10000)
	manager.WorkerQueue = make(chan chan SearchRequest, workersNumber)

	for i := 0; i < workersNumber; i++ {
		worker := Worker{
			DataSources: ds,
			Work:        make(chan SearchRequest),
			WorkerQueue: manager.WorkerQueue,
			Storage:     manager.Storage,
			Queue:		 manager.Queue,
		}
		worker.Start()
		go func() {
			for {
				select {
				case work := <-manager.Queue:
					go func() {
						worker := <-manager.WorkerQueue
						worker <- work
					}()
				}
			}
		}()
	}
	return nil
}

func (manager *Manager) readDataSources(path string) ([]DataSource, error) {
	var ds []DataSource
	if path == "" {
		return ds, errors.New("path to data source was not specified")
	}
	file, err := os.Open(path)
	if err != nil {
		return ds, err
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&ds)
	if err != nil {
		return ds, err
	}
	return ds, nil
}

func (manager *Manager) StartCrawling(req SearchRequest) error {
	fieldsPart := map[string][]string{}
	var dataSource DataSource
	for _, ds := range manager.DataSources {
		if ds.Name == req.SourceName {
			dataSource = ds
			break
		}
	}
	if dataSource.Name == "" {
		return errors.New("incorrect data source name specified")
	}
	firstKey := ""
	for key, value := range req.Fields {
		if firstKey == "" {
			firstKey = key
		}
		checkDs := false
		for _, dsField := range dataSource.Keys {
			if key == dsField {
				checkDs = true
				break
			}
		}
		if !checkDs {
			return errors.New("key " + key + " was not found in data source " + dataSource.Name)
		}
		fieldsPart[key] = []string{}
		setParts := strings.Split(value, ",")
		if len(setParts) > 1 || key == "query" {
			fieldsPart[key] = setParts
		} else {
			rangeParts := strings.Split(value, "-")
			if len(rangeParts) != 2 {
				fieldsPart[key] = setParts
				// return errors.New("search range for key " + key + " was specified incorrectly: " + value)
			} else {
				start, err := strconv.Atoi(rangeParts[0])
				if err != nil {
					return err
				}
				finish, err := strconv.Atoi(rangeParts[1])
				if err != nil {
					return err
				}
				if start > finish {
					return errors.New("range error for key " + key + ": start value must be less or equal than finish value")
				}
				rangeSlice := make([]string, finish-start+1)
				for i := range rangeSlice {
					val := strconv.Itoa(start + i)
					rangeSlice[i] = val
				}
				fieldsPart[key] = rangeSlice
			}
		}
	}

	pagesField := []map[string]string{}
	if req.ID == "" {
		pagesField = manager.formPagesSearchField(req, dataSource)
	}
	for _, f := range pagesField {
		workerReq := SearchRequest{SourceName: req.SourceName, Source: dataSource, Fields: f}
		manager.Queue <- workerReq
	}
	return nil
}

func (manager *Manager) getMaxResults(req SearchRequest, source DataSource, conf config.Configuration) (int, error){
	path := source.Path
	data, err := query.MakeQuery(path, "", req.Fields, 15, manager.Storage, conf)
	if err != nil{
		return 0, err
	}
	log.Println(data)
	js := gjson.Parse(data)
	jj := js.Get("search-results")
	jw := jj.Get("opensearch:totalResults").Str
	total, err := strconv.Atoi(jw)
	if err != nil{
		return 0, err
	}
	return total , nil
}

func (manager *Manager)formPagesSearchField(req SearchRequest, source DataSource) []map[string]string {
	conf, _ := config.ReadConfig("config.json")
	conf.InitKeys("keys.txt")
	maxSearchResults, err := manager.getMaxResults(req, source, conf)
	if err != nil{
		log.Println(err)
		return nil
	}
	maxPages := min(maxSearchResults/conf.ResultsPerPage, 4975/conf.ResultsPerPage)
	result := make([]map[string]string, maxPages)
	counter := 0
	for i:=0; i < maxPages; i++ {
		item := make(map[string]string, len(req.Fields)+1)
		for k, v := range req.Fields{
			item[k] = v
		}
		item["start"] = strconv.Itoa(counter)
		result[i] = item
		counter+=conf.ResultsPerPage
	}
	return result
}

func min(a int, b int) int{
	if a > b {
		return b
	}
	return a
}

func formSearchField(fields map[string][]string, key string, currentMap []map[string]string) []map[string]string {
	field := fields[key]
	newMap := []map[string]string{}
	for _, f := range field {
		val := f
		if len(currentMap) > 0 {
			for _, v := range currentMap {
				newElem := map[string]string{}
				for vKey, vVal := range v {
					newElem[vKey] = vVal
				}
				newElem[key] = val
				newMap = append(newMap, newElem)
			}
		} else {
			newElem := map[string]string{}
			newElem[key] = f
			newMap = append(newMap, newElem)
		}
	}
	result := newMap
	cont := false
	for k := range fields {
		if cont {
			result = formSearchField(fields, k, newMap)
		}
		if k == key {
			cont = true
		}
	}
	return result
}