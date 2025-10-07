package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/nisimpson/jsonapi/v2"
)

// Article represents a blog article with relationships to author, tags, and comments
type Article struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	AuthorID string   `json:"-"`
	TagIDs   []string `json:"-"`
}

func (a Article) ResourceID() string   { return a.ID }
func (a Article) ResourceType() string { return "articles" }

func (a *Article) SetResourceID(id string) error {
	a.ID = id
	return nil
}

func (a Article) MarshalLinks() map[string]jsonapi.Link {
	return map[string]jsonapi.Link{
		"self": {Href: fmt.Sprintf("/articles/%s", a.ID)},
	}
}

func (a *Article) UnmarshalLinks(links map[string]jsonapi.Link) error {
	// Links are read-only in this example
	return nil
}

func (a Article) Relationships() map[string]jsonapi.RelationType {
	return map[string]jsonapi.RelationType{
		"author":   jsonapi.RelationToOne,     // Single related resource
		"tags":     jsonapi.RelationToMany,    // Multiple related resources
		"comments": jsonapi.RelationLinksOnly, // Links only, no data included
	}
}

func (a Article) MarshalRef(name string) []jsonapi.ResourceIdentifier {
	switch name {
	case "author":
		return jsonapi.OneRef(User{ID: a.AuthorID})
	case "tags":
		var tags []Tag
		for _, tagID := range a.TagIDs {
			tags = append(tags, Tag{ID: tagID})
		}
		return jsonapi.ManyRef(tags...)
	}
	return nil
}

func (a Article) RelationLinks(name string) map[string]jsonapi.Link {
	switch name {
	case "comments":
		return map[string]jsonapi.Link{
			"self":    {Href: fmt.Sprintf("/articles/%s/relationships/comments", a.ID)},
			"related": {Href: fmt.Sprintf("/articles/%s/comments", a.ID)},
		}
	}
	return nil
}

func (a *Article) SetRelation(name, id string, meta map[string]interface{}) error {
	switch name {
	case "author":
		a.AuthorID = id
	case "tags":
		a.TagIDs = append(a.TagIDs, id)
	}
	return nil
}

// User represents an article author
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (u User) ResourceID() string   { return u.ID }
func (u User) ResourceType() string { return "users" }

func (u *User) SetResourceID(id string) error {
	u.ID = id
	return nil
}

// Tag represents an article tag
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (t Tag) ResourceID() string   { return t.ID }
func (t Tag) ResourceType() string { return "tags" }

func (t *Tag) SetResourceID(id string) error {
	t.ID = id
	return nil
}

// In-memory storage
var (
	articles = map[string]Article{
		"1": {
			ID:       "1",
			Title:    "Getting Started with JSON:API",
			Content:  "JSON:API is a specification for building APIs in JSON...",
			AuthorID: "1",
			TagIDs:   []string{"1", "2"},
		},
		"2": {
			ID:       "2",
			Title:    "Advanced JSON:API Relationships",
			Content:  "Understanding relationships in JSON:API...",
			AuthorID: "1",
			TagIDs:   []string{"1"},
		},
	}

	users = map[string]User{
		"1": {ID: "1", Name: "John Doe"},
	}

	tags = map[string]Tag{
		"1": {ID: "1", Name: "tutorial"},
		"2": {ID: "2", Name: "jsonapi"},
	}
)

// Article handlers
func getArticle(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	article, exists := articles[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	req.Marshal(w, http.StatusOK, article)
}

func listArticles(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())
	var articleList []Article
	for _, article := range articles {
		articleList = append(articleList, article)
	}

	req.Marshal(w, http.StatusOK, articleList)
}

func createArticle(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	var article Article
	err := req.Unmarshal(r.Body, &article)
	if err != nil {
		req.MarshalErrors(w, http.StatusBadRequest, err)
		return
	}

	// Generate ID
	article.ID = strconv.Itoa(len(articles) + 1)
	articles[article.ID] = article

	req.Marshal(w, http.StatusCreated, article)
}

func updateArticle(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	article, exists := articles[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	err := req.Unmarshal(r.Body, &article)
	if err != nil {
		req.MarshalErrors(w, http.StatusBadRequest, err)
		return
	}

	articles[req.ResourceID] = article
	req.Marshal(w, http.StatusOK, article)
}

func deleteArticle(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	if _, exists := articles[req.ResourceID]; !exists {
		http.NotFound(w, r)
		return
	}

	delete(articles, req.ResourceID)
	w.WriteHeader(http.StatusNoContent)
}

// Relationship handlers for articles
func getArticleAuthor(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	article, exists := articles[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	// Use MarshalRef to return relationship data (not the full user resource)
	_, err := req.MarshalRef(w, http.StatusOK, "author", article)
	if err != nil {
		// Don't write to response again - just log the error
		log.Printf("Failed to marshal author relationship: %v", err)
	}
}

func updateArticleAuthor(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	article, exists := articles[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	err := req.UnmarshalRef(r.Body, "author", &article)
	if err != nil {
		req.MarshalErrors(w, http.StatusBadRequest, err)
		return
	}

	articles[req.ResourceID] = article
	w.WriteHeader(http.StatusNoContent)
}

func getArticleTags(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	article, exists := articles[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	var articleTags []Tag
	for _, tagID := range article.TagIDs {
		if tag, exists := tags[tagID]; exists {
			articleTags = append(articleTags, tag)
		}
	}
	req.Marshal(w, http.StatusOK, articleTags)
}

func addArticleTag(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	article, exists := articles[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	err := req.UnmarshalRef(r.Body, "tags", &article)
	if err != nil {
		req.MarshalErrors(w, http.StatusBadRequest, err)
		return
	}

	articles[req.ResourceID] = article
	w.WriteHeader(http.StatusNoContent)
}

func removeArticleTag(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	article, exists := articles[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	// Create a temporary article to capture what should be removed
	var tempArticle Article
	err := req.UnmarshalRef(r.Body, "tags", &tempArticle)
	if err != nil {
		req.MarshalErrors(w, http.StatusBadRequest, err)
		return
	}

	// Remove the tags that were specified in the request
	for _, removeTagID := range tempArticle.TagIDs {
		for i, tagID := range article.TagIDs {
			if tagID == removeTagID {
				article.TagIDs = append(article.TagIDs[:i], article.TagIDs[i+1:]...)
				break
			}
		}
	}

	articles[req.ResourceID] = article
	w.WriteHeader(http.StatusNoContent)
}

func getArticleComments(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())
	// Return empty array for demo - comments would be fetched from database
	req.Marshal(w, http.StatusOK, []interface{}{})
}

// User handlers
func getUser(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	user, exists := users[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	req.Marshal(w, http.StatusOK, user)
}

func listUsers(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())
	var userList []User
	for _, user := range users {
		userList = append(userList, user)
	}

	req.Marshal(w, http.StatusOK, userList)
}

// Tag handlers
func getTag(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())

	tag, exists := tags[req.ResourceID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	req.Marshal(w, http.StatusOK, tag)
}

func listTags(w http.ResponseWriter, r *http.Request) {
	req := jsonapi.FromContext(r.Context())
	var tagList []Tag
	for _, tag := range tags {
		tagList = append(tagList, tag)
	}

	req.Marshal(w, http.StatusOK, tagList)
}

// Logging middleware
func loggingMiddleware(w http.ResponseWriter, r *http.Request, next http.Handler) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	next.ServeHTTP(w, r)
}

func main() {

	// Create resource handlers
	articleHandler := jsonapi.ResourceHandler{
		Retrieve: http.HandlerFunc(getArticle),
		List:     http.HandlerFunc(listArticles),
		Create:   http.HandlerFunc(createArticle),
		Update:   http.HandlerFunc(updateArticle),
		Delete:   http.HandlerFunc(deleteArticle),

		// Create relationship handlers for articles
		Refs: jsonapi.RelationshipHandlerMux{
			"author": jsonapi.RelationshipHandler{
				Get:    http.HandlerFunc(getArticleAuthor),
				Update: http.HandlerFunc(updateArticleAuthor),
			},
			"tags": jsonapi.RelationshipHandler{
				Get: http.HandlerFunc(getArticleTags),
				Add: http.HandlerFunc(addArticleTag),
				Del: http.HandlerFunc(removeArticleTag),
			},
			"comments": jsonapi.RelationshipHandler{
				Get: http.HandlerFunc(getArticleComments),
			},
		},
	}

	userHandler := jsonapi.ResourceHandler{
		Retrieve: http.HandlerFunc(getUser),
		List:     http.HandlerFunc(listUsers),
	}

	tagHandler := jsonapi.ResourceHandler{
		Retrieve: http.HandlerFunc(getTag),
		List:     http.HandlerFunc(listTags),
	}

	// Register handlers
	handlers := map[string]jsonapi.ResourceHandler{
		"articles": articleHandler,
		"users":    userHandler,
		"tags":     tagHandler,
	}

	// Create server with middleware
	mux := jsonapi.DefaultServeMux(handlers, jsonapi.MiddlewareFunc(loggingMiddleware))

	// Add a simple index page
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head><title>JSON:API Example Server</title></head>
<body>
	<h1>JSON:API Example Server</h1>
	<h2>Available Endpoints:</h2>
	<ul>
		<li><a href="/articles">GET /articles</a> - List all articles</li>
		<li><a href="/articles/1">GET /articles/1</a> - Get article 1</li>
		<li><a href="/articles/1/tags">GET /articles/1/tags</a> - Get article 1 tags</li>
		<li><a href="/articles/1/author">GET /articles/1/author</a> - Get article 1 author</li>
		<li><a href="/users">GET /users</a> - List all users</li>
		<li><a href="/tags">GET /tags</a> - List all tags</li>
	</ul>
	<h3>Relationship Operations:</h3>
	<ul>
		<li>PATCH /articles/1/relationships/author - Update article author</li>
		<li>POST /articles/1/relationships/tags - Add tags to article</li>
		<li>DELETE /articles/1/relationships/tags - Remove tags from article</li>
	</ul>
	<p>Try: <code>curl -H "Accept: application/vnd.api+json" http://localhost:8080/articles</code></p>
</body>
</html>`)
	})

	log.Println("Starting JSON:API example server on :8080")
	log.Println("Visit http://localhost:8080 for available endpoints")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
