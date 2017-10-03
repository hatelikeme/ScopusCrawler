package crawler

import (
	"errors"
	"../config"
	"../logger"
	"../models"
	"../query"
	"../storage"
	"github.com/tidwall/gjson"
	"strconv"
	"strings"
	"hash/fnv"
	"log"
)

type Worker struct {
	Config      config.Configuration
	Storage     storage.MySqlStorage
	DataSources []DataSource
	Work        chan SearchRequest
	WorkerQueue chan chan SearchRequest
	Queue       chan SearchRequest
}

func (worker *Worker) Start() {
	go func() {
		worker.Config, _ = config.ReadConfig("config.json")
		worker.Config.InitKeys("keys.txt")
		for {
			worker.WorkerQueue <- worker.Work
			work := <-worker.Work
			switch work.SourceName {
			case "affiliation":
				affiliation, err := worker.GetAffiliation(work)
				if err == nil {
					worker.Storage.CreateAffiliation(affiliation)
				} else {
					log.Println(err)
				}
			case "PagesNum":
				requests := worker.formPagesSearchField(work)
				for _, req := range requests {
					req.SourceName = "search"
					worker.Queue <- req
				}
			case "search":
				source, err := worker.extractSource("search")
				if err != nil {
					logger.Error.Println(err)
					return
				}
				data, err := query.MakeQuery(source.Path, "", work.Fields, worker.Config.RequestTimeout, worker.Storage, worker.Config)
				if err != nil {
					logger.Error.Println(err)
					return
				}
				articles, err := worker.ExtractArticles(data)
				if err != nil {
					logger.Error.Println(err)
					return
				}
				var articleDs DataSource
				for _, v := range worker.DataSources {
					if v.Name == "article" {
						articleDs = v
						break
					}
				}
				for _, article := range articles {
					worker.Queue <- SearchRequest{"article", articleDs, article.ScopusID, nil}
				}
			case "article":
				art := models.Article{ScopusID: work.ID}
				err := worker.ProceedArticle(&art, work.Source, 0)
				if err != nil {
					logger.Error.Println(err)
					return
				}
			}
		}
	}()
}

func (worker *Worker) GetAffiliation(req SearchRequest) (models.Affiliation, error) {
	source, err := worker.extractSource(req.SourceName)
	if err != nil {
		return models.Affiliation{}, err
	}
	data, err := query.MakeQuery(source.Path, req.ID, req.Fields, worker.Config.RequestTimeout, worker.Storage, worker.Config)
	if err != nil {
		return models.Affiliation{}, err
	}
	affildata := gjson.Get(data, "affiliation-retreival-response")
	result := models.Affiliation{}
	result.ScopusID = req.ID
	add := affildata.Get("address")
	if add.Exists() {
		result.Address = add.String()
	}
	city := affildata.Get("city")
	if city.Exists() {
		result.City = city.String()
	}
	country := affildata.Get("country")
	if country.Exists() {
		result.Country = country.String()
	}
	title := affildata.Get("affiliation-name")
	if title.Exists() {
		result.Title = title.String()
	}
	return result, nil
}

func (worker *Worker) getMaxResults(req SearchRequest) (int, error) {
	source, err := worker.extractSource(req.SourceName)
	if err != nil {
		return 0, err
	}
	data, err := query.MakeQuery(source.Path, "", req.Fields, worker.Config.RequestTimeout, worker.Storage, worker.Config)
	if err != nil {
		return 0, err
	}
	log.Println(data)
	js := gjson.Parse(data)
	jj := js.Get("search-results")
	jw := jj.Get("opensearch:totalResults").String()
	total, err := strconv.Atoi(jw)
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (worker *Worker) formPagesSearchField(req SearchRequest) []SearchRequest {
	maxSearchResults, err := worker.getMaxResults(req)
	if err != nil {
		log.Println(err)
		return nil
	}
	maxPages := min(maxSearchResults/worker.Config.ResultsPerPage, 4975/worker.Config.ResultsPerPage)
	result := make([]map[string]string, maxPages+1)
	counter := 0
	for i := 0; i < maxPages+1; i++ {
		item := make(map[string]string, len(req.Fields)+1)
		for k, v := range req.Fields {
			item[k] = v
		}
		item["start"] = strconv.Itoa(counter)
		result[i] = item
		counter += worker.Config.ResultsPerPage
	}
	rv := []SearchRequest{}
	for _, f := range result {
		workerReq := SearchRequest{SourceName: req.SourceName, Source: req.Source, Fields: f}
		rv = append(rv, workerReq)
	}
	return rv
}

func ExtractEntry(entry gjson.Result, article *models.Article) {
	entry = entry.Get("coredata")
	ScopusID := entry.Get("dc:identifier")
	if ScopusID.Exists() {
		article.ScopusID = strings.Replace(ScopusID.Str, "SCOPUS_ID:", "", 1)
	}
	title := entry.Get("dc:title")
	if title.Exists() {
		article.Title = title.Str
	}
	citedby := entry.Get("citedby-count")
	if citedby.Exists() {
		article.CitationsCount = int(citedby.Int())
	}
	pubdate := entry.Get("prism:coverDate")
	if pubdate.Exists() {
		article.PublicationDate = pubdate.Str
	}
	pubtype := entry.Get("prism:aggregationType")
	if pubtype.Exists() {
		article.PublicationType = pubtype.Str
	}
	pubname := entry.Get("prism:publicationName")
	if pubname.Exists() {
		article.PublicationTitle = pubname.Str
	}
	abstracts := entry.Get("dc:description")
	if abstracts.Exists() {
		article.Abstracts = abstracts.Str
	}
}

func ExtractAuthors(entry gjson.Result, article *models.Article) {
	entry = entry.Get("authors")
	authors := []models.Author{}
	for _, author := range entry.Get("author").Array() {
		aut := models.Author{}
		aScopusID := author.Get("@auid")
		if aScopusID.Exists() {
			aut.ScopusID = aScopusID.Str
		}
		name := author.Get("preferred-name.ce:given-name")
		if name.Exists() {
			aut.Name = name.Str
		}
		surname := author.Get("ce:surname")
		if surname.Exists() {
			aut.Surname = surname.Str
		}
		givenName := author.Get("ce:indexed-name")
		if givenName.Exists() {
			aut.IndexedName = givenName.Str
		}
		initials := author.Get("ce:initials")
		if initials.Exists() {
			aut.Initials = initials.Str
		}
		afid := author.Get("affiliation.@id")
		if afid.Exists() {
			aut.AffiliationID = afid.Str
		}
		authors = append(authors, aut)
	}
	article.Authors = authors
}

func ExtractAffiliation(entry gjson.Result, article *models.Article) {
	affiliation := []models.Affiliation{}
	for _, res := range entry.Get("affiliation").Array() {
		aff := models.Affiliation{}
		afid := res.Get("@id")
		if afid.Exists() {
			aff.ScopusID = afid.Str
		}
		affname := res.Get("affilname")
		if affname.Exists() {
			aff.Title = affname.Str
		}
		affcity := res.Get("affiliation-city")
		if affcity.Exists() {
			aff.City = affcity.Str
		}
		affcountry := res.Get("affiliation-country")
		if affcountry.Exists() {
			aff.Country = affcountry.Str
		}
		affiliation = append(affiliation, aff)
	}
	article.Affiliations = affiliation
}

func ExtractScopusID(entry gjson.Result) (article models.Article) {
	ScopusID := entry.Get("dc:identifier")
	if ScopusID.Exists() {
		article.ScopusID = strings.Replace(ScopusID.Str, "SCOPUS_ID:", "", 1)
	}
	return article
}

func (worker *Worker) ExtractArticles(rawResponse string) ([]models.Article, error) {
	result := []models.Article{}
	sresults := gjson.Get(rawResponse, "search-results")

	if sresults.Exists() {
		for _, entry := range sresults.Get("entry").Array() {
			article := ExtractScopusID(entry)
			result = append(result, article)
		}
		return result, nil
	} else {
		return nil, errors.New("empty search response")
	}
}

func ExtractKeywords(response gjson.Result, article *models.Article) {
	for _, keyword := range response.Get("authkeywords.author-keyword").Array() {
		kw := models.Keyword{}
		kw.Value = keyword.Get("$").Str
		h := fnv.New64a()
		h.Write([]byte(kw.Value))
		kw.ID = strconv.Itoa(int(h.Sum64()))
		article.Keywords = append(article.Keywords, kw)
	}
}

func ExtractSubjectArea(response gjson.Result, article *models.Article) {
	for _, subarea := range response.Get("subject-areas.subject-area").Array() {
		subjectarea := models.SubjectArea{}
		title := subarea.Get("@abbrev")
		if title.Exists() {
			subjectarea.Title = title.Str
		}
		code := subarea.Get("@code")
		if code.Exists() {
			subjectarea.Code = subarea.Str
		}
		desc := subarea.Get("$")
		if desc.Exists() {
			subjectarea.Description = desc.Str
		}
		hash := fnv.New64a()
		hash.Write([]byte(subjectarea.Code + subjectarea.Description + subjectarea.Title))
		subjectarea.ScopusID = strconv.Itoa(int(hash.Sum64()))
		article.SubjectAreas = append(article.SubjectAreas, subjectarea)
	}
}

func ExtractRefAuthors(refinfo gjson.Result) (authors []models.Author) {
	for _, refaut := range refinfo.Get("ref-authors.author").Array() {
		author := models.Author{}
		initials := refaut.Get("ce:initials")
		if initials.Exists() {
			author.Initials = initials.Str
		}
		indexedName := refaut.Get("ce:indexed-name")
		if indexedName.Exists() {
			author.IndexedName = indexedName.Str
		}
		surname := refaut.Get("ce:surname")
		if surname.Exists() {
			author.Surname = surname.Str
		}
		authors = append(authors, author)
	}
	return authors
}

func ExtractReferences(response gjson.Result) ([]models.Article) {
	records := []models.Article{}
	for _, bibrecord := range response.Get("item.bibrecord.tail.bibliography.reference").Array() {
		record := models.Article{}
		refinfo := bibrecord.Get("ref-info")
		title := refinfo.Get("ref-sourcetitle")
		if title.Exists() {
			record.Title = title.Str
		}
		year := refinfo.Get("ref-publicationyear")
		if year.Exists() {
			record.PublicationDate = year.Str
		}
		scopusID := refinfo.Get("refd-itemidlist.itemid.$")
		if scopusID.Exists() {
			record.ScopusID = strings.Replace(scopusID.Str, "SCOPUS_ID:", "", 1)
		}
		record.Authors = ExtractRefAuthors(refinfo)
		records = append(records, record)
	}
	return records
}

func flattenAffiliations(aff []models.Affiliation) []string {
	rv := []string{}
	for _, a := range aff {
		rv = append(rv, a.ScopusID)
	}
	return rv
}

func checkIfIn(slice []string, elem string) bool {
	for _, el := range slice {
		if elem == el {
			return true
		}
	}
	return false
}

func (worker *Worker) extractSource(sourceName string) (DataSource, error) {
	for _, ds := range worker.DataSources {
		if ds.Name == sourceName {
			return ds, nil
		}
	}
	return DataSource{}, errors.New("data source not found")
}

func (worker *Worker) CheckAffiliations(article *models.Article) error {
	for _, aff := range article.Authors{
		r, err := worker.Storage.CheckAffiliation(aff.AffiliationID)
		if err != nil{
			return err
		}
		if r {
			source, err := worker.extractSource("affiliation")
			if err != nil {
				return err
			}
			worker.Queue <- SearchRequest{SourceName: "affiliation", Source: source, ID: aff.AffiliationID}
		}
	}
	return nil
}

func (worker *Worker) ProceedArticle(article *models.Article, articleDs DataSource, depth int) error {
	source, err := worker.extractSource("article")
	if err != nil{
		return err
	}
	articleData, err := query.MakeQuery(source.Path, article.ScopusID, map[string]string{}, worker.Config.RequestTimeout,
		worker.Storage, worker.Config)
	if err != nil {
		logger.Error.Println("Error on requesting data for id=" + article.ScopusID)
		logger.Error.Println(err)
	}
	response := gjson.Get(articleData, "abstracts-retrieval-response")
	ExtractAffiliation(response, article)
	ExtractEntry(response, article)
	ExtractAuthors(response, article)
	ExtractKeywords(response, article)
	ExtractSubjectArea(response, article)
	references := ExtractReferences(response)
	if depth < worker.Config.ReferencesDepth {
		for _, ref := range references {
			worker.ProceedArticle(&ref, articleDs, depth+1)
			article.References = append(article.References, ref)
		}
	}
	worker.CheckAffiliations(article)
	err = worker.Storage.CreateArticle(*article)
	if err != nil {
		return errors.New("Error writing article to database")
	}
	return nil
}
