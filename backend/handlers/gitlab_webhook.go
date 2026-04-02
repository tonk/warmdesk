package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	appws "github.com/tonk/warmdesk/ws"
)

// ── GitLab payload structs ────────────────────────────────────────────────────

type gitlabUser struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

func (u gitlabUser) display() string {
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}

type gitlabCommit struct {
	ID      string     `json:"id"`
	Message string     `json:"message"`
	URL     string     `json:"url"`
	Author  gitlabUser `json:"author"`
}

func (c gitlabCommit) shortID() string {
	if len(c.ID) > 7 {
		return c.ID[:7]
	}
	return c.ID
}

func (c gitlabCommit) firstLine() string {
	msg := strings.TrimSpace(c.Message)
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return msg[:i]
	}
	return msg
}

type gitlabObjectAttrs struct {
	IID         int    `json:"iid"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	State       string `json:"state"`   // "opened", "closed", "merged"
	Action      string `json:"action"`  // "open", "close", "merge", "update"
	Description string `json:"description"`
}

type gitlabPayload struct {
	ObjectKind  string            `json:"object_kind"`
	Ref         string            `json:"ref"`
	Commits     []gitlabCommit    `json:"commits"`
	UserName    string            `json:"user_name"`
	User        gitlabUser        `json:"user"`
	Repository  struct {
		Name     string `json:"name"`
		Homepage string `json:"homepage"`
	} `json:"repository"`
	Project struct {
		Name              string `json:"name"`
		WebURL            string `json:"web_url"`
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes gitlabObjectAttrs `json:"object_attributes"`
}

func (p gitlabPayload) repoName() string {
	if p.Project.PathWithNamespace != "" {
		return p.Project.PathWithNamespace
	}
	return p.Repository.Name
}

func (p gitlabPayload) repoURL() string {
	if p.Project.WebURL != "" {
		return p.Project.WebURL
	}
	return p.Repository.Homepage
}

func (p gitlabPayload) pusher() string {
	if p.UserName != "" {
		return p.UserName
	}
	return p.User.display()
}

// ── Chat message formatting ───────────────────────────────────────────────────

func formatGitLabEvent(p gitlabPayload) string {
	repo := p.repoName()
	repoLink := p.repoURL()

	switch p.ObjectKind {
	case "push":
		branch := p.Ref
		if strings.HasPrefix(branch, "refs/heads/") {
			branch = branch[len("refs/heads/"):]
		}
		n := len(p.Commits)
		if n == 0 {
			return ""
		}
		word := "commit"
		if n != 1 {
			word = "commits"
		}
		lines := []string{
			fmt.Sprintf("**%s** pushed %d %s to **%s** ([%s](%s))",
				p.pusher(), n, word, branch, repo, repoLink),
		}
		limit := n
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			c := p.Commits[i]
			lines = append(lines, fmt.Sprintf("• [`%s`](%s) %s", c.shortID(), c.URL, c.firstLine()))
		}
		if n > 5 {
			lines = append(lines, fmt.Sprintf("• … and %d more", n-5))
		}
		return strings.Join(lines, "\n")

	case "merge_request":
		oa := p.ObjectAttributes
		action := oa.Action
		if action == "merge" {
			action = "merged"
		}
		return fmt.Sprintf("**%s** %s merge request [!%d %s](%s) in [%s](%s)",
			p.User.display(), action, oa.IID, oa.Title, oa.URL, repo, repoLink)

	case "issue":
		oa := p.ObjectAttributes
		action := oa.Action
		return fmt.Sprintf("**%s** %s issue [#%d %s](%s) in [%s](%s)",
			p.User.display(), action, oa.IID, oa.Title, oa.URL, repo, repoLink)

	default:
		return ""
	}
}

// ── Card link extraction ──────────────────────────────────────────────────────

func linkGitLabCards(hook models.ProjectWebhook, p gitlabPayload) {
	repo := p.repoName()

	switch p.ObjectKind {
	case "push":
		for _, commit := range p.Commits {
			links := services.LinkCardsFromText(commit.Message, models.CardLink{
				Platform:  "gitlab",
				LinkType:  "commit",
				Title:     commit.firstLine(),
				URL:       commit.URL,
				Reference: commit.ID,
				Author:    commit.Author.display(),
				Status:    "merged",
				RepoName:  repo,
			})
			for _, l := range links {
				appws.BroadcastToProject(hook.ProjectID, appws.Message{
					Type:    appws.TypeCardLinkCreated,
					Payload: l,
				})
			}
		}

	case "merge_request":
		oa := p.ObjectAttributes
		status := oa.State
		if status == "merged" {
			status = "merged"
		} else if status == "closed" {
			status = "closed"
		} else {
			status = "open"
		}
		links := services.LinkCardsFromText(oa.Title, models.CardLink{
			Platform:  "gitlab",
			LinkType:  "pr",
			Title:     oa.Title,
			URL:       oa.URL,
			Reference: fmt.Sprintf("%d", oa.IID),
			Author:    p.User.display(),
			Status:    status,
			RepoName:  repo,
		})
		for _, l := range links {
			appws.BroadcastToProject(hook.ProjectID, appws.Message{
				Type:    appws.TypeCardLinkCreated,
				Payload: l,
			})
		}

	case "issue":
		oa := p.ObjectAttributes
		status := oa.State
		if status == "closed" {
			status = "closed"
		} else {
			status = "open"
		}
		links := services.LinkCardsFromText(oa.Title, models.CardLink{
			Platform:  "gitlab",
			LinkType:  "issue",
			Title:     oa.Title,
			URL:       oa.URL,
			Reference: fmt.Sprintf("%d", oa.IID),
			Author:    p.User.display(),
			Status:    status,
			RepoName:  repo,
		})
		for _, l := range links {
			appws.BroadcastToProject(hook.ProjectID, appws.Message{
				Type:    appws.TypeCardLinkCreated,
				Payload: l,
			})
		}
	}
}

// ── Handler ───────────────────────────────────────────────────────────────────

// IncomingGitLabWebhook POST /api/v1/gitlab-webhook/:token (public)
func IncomingGitLabWebhook(c *gin.Context) {
	token := c.Param("token")

	var hook models.ProjectWebhook
	if err := database.DB.Where("token = ? AND type = ?", token, "gitlab").First(&hook).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// GitLab sends the secret in X-Gitlab-Token header as a plain string.
	gitlabToken := c.GetHeader("X-Gitlab-Token")
	if gitlabToken != "" && gitlabToken != hook.Token {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	var payload gitlabPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	// Post chat message.
	text := formatGitLabEvent(payload)
	if text != "" {
		msg := models.ChatMessage{
			ProjectID: hook.ProjectID,
			UserID:    0,
			Body:      text,
			IsBot:     true,
			BotName:   hook.Name,
		}
		if err := database.DB.Create(&msg).Error; err == nil {
			appws.BroadcastToProject(hook.ProjectID, appws.Message{
				Type:    appws.TypeChatMessageCreated,
				Payload: msg,
			})
		}
	}

	// Extract card links.
	linkGitLabCards(hook, payload)

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
