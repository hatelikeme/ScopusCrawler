package models

type Coredata struct{
	scopusID []string `json: "dc:identifier"`
	citationCount int `json:"citedby-count"`
	title []string		`json: "dc:title"`
	publicationType []string `json: "prism:aggregationType"`
	publicationDate []string `json: "prism:coverDate"`
	abstract []string `json: "dc:description"`
	affiliation Affiliation
}

type Request struct {
	sourceName string `json: "sourceName"`
	fields Fields
}

type Fields struct {

}

