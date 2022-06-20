package patching

type Package struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Categories     []string `json:"categories"`
	CategoryIDs    []string `json:"category_ids"`
	KBArticleIDs   []string `json:"kb_article_ids"`
	MoreInfoURLs   []string `json:"more_info_urls"`
	SupportURL     string   `json:"support_url"`
	UpdateID       string   `json:"guid"`
	RevisionNumber int32    `json:"revision_number"`
	Severity       string   `json:"severity"`
	Installed      bool     `json:"installed"`
	Downloaded     bool     `json:"downloaded"`
}
