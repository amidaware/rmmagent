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

type WinUpdateInstallResult struct {
	AgentID  string `json:"agent_id"`
	UpdateID string `json:"guid"`
	Success  bool   `json:"success"`
}

type SupersededUpdate struct {
	AgentID  string `json:"agent_id"`
	UpdateID string `json:"guid"`
}

type AgentNeedsReboot struct {
	AgentID     string `json:"agent_id"`
	NeedsReboot bool   `json:"needs_reboot"`
}
