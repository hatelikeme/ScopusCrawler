package models

type Author struct {
	ScopusID      string    `json:"@auid"`
	Initials      string    `json:"ce:initials"`
	IndexedName   string    `json:"ce:indexed-name"`
	Surname       string    `json:"ce:surname"`
	Name          string
	AffiliationID string
	Affiliation   Affiliation
}

type Affiliation struct {
	ScopusID   string `json:"@id"`
	Title      string `json:"affilname"`
	Country    string `json:"affiliation-country"`
	City       string `json:"affiliation-city"`
	State      string
	PostalCode string
	Address    string
}

type Article struct {
	ScopusID         string    `json:"dc:identifier"`
	Title            string    `json:"dc:title"`
	Abstracts        string    `json:"dc:description"`
	PublicationDate  string    `json:"prism:coverDate"`
	CitationsCount   int    `json:"citedby-count"`
	PublicationType  string    `json:"prism:aggregationType"`
	PublicationTitle string    `json:"prism:coverDate"`
	Affiliations     []Affiliation    `json:"affiliation"`
	Authors          []Author    `json:"authors"`
	Keywords         []Keyword    `json:"authkeywords"`
	SubjectAreas     []SubjectArea `json:"subject-areas"`
	References       []Article    `json:"references"`
}

type SubjectArea struct {
	ScopusID    string `json:"@code"`
	Title       string    `json:"@abbrev"`
	Code        string    `json:"@code"`
	Description string    `json:"@_fa"`
}

type Keyword struct {
	ID    string
	Value string
}
