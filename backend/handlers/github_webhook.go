package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/models"
	"github.com/tonk/coworker/services"
	appws "github.com/tonk/coworker/ws"
)

// ── GitHub payload structs ────────────────────────────────────────────────────

type githubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

func (u githubUser) display() string {
	if u.Name != "" {
		return u.Name
	}
	return u.Login
}

type githubRepository struct {
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

type githubCommit struct {
	ID      string     `json:"id"`
	Message string     `json:"message"`
	URL     string     `json:"url"`
	Author  githubUser `json:"author"`
}

func (c githubCommit) shortID() string {
	if len(c.ID) > 7 {
		return c.ID[:7]
	}
	return c.ID
}

func (c githubCommit) firstLine() string {
	msg := strings.TrimSpace(c.Message)
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return msg[:i]
	}
	return msg
}

type githubPullRequest struct {
	Number  int        `json:"number"`
	Title   string     `json:"title"`
	HTMLURL string     `json:"html_url"`
	User    githubUser `json:"user"`
	Merged  bool       `json:"merged"`
	State   string     `json:"state"` // "open" | "closed"
	Head    struct {
		Label string `json:"label"`
	} `json:"head"`
	Base struct {
		Label string `json:"label"`
	} `json:"base"`
}

type githubIssue struct {
	Number  int        `json:"number"`
	Title   string     `json:"title"`
	HTMLURL string     `json:"html_url"`
	User    githubUser `json:"user"`
	State   string     `json:"state"` // "open" | "closed"
}

type githubPayload struct {
	Ref         string            `json:"ref"`
	Commits     []githubCommit    `json:"commits"`
	HeadCommit  *githubCommit     `json:"head_commit"`
	Pusher      githubUser        `json:"pusher"`
	Sender      githubUser        `json:"sender"`
	Repository  githubRepository  `json:"repository"`
	Action      string            `json:"action"`
	PullRequest *githubPullRequest `json:"pull_request"`
	Issue       *githubIssue      `json:"issue"`
	RefType     string            `json:"ref_type"`
}

// ── Signature verification ────────────────────────────────────────────────────

func verifyGitHubSignature(secret string, body []byte, sig string) bool {
	if sig == "" {
		return true // no secret configured on their side
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// ── Chat message formatting ───────────────────────────────────────────────────

func formatGitHubEvent(event string, p githubPayload) string {
	repo := p.Repository.FullName
	repoLink := p.Repository.HTMLURL

	switch event {
	case "push":
		branch := p.Ref
		if strings.HasPrefix(branch, "refs/heads/") {
			branch = branch[len("refs/heads/"):]
		}
		pusher := p.Pusher.display()
		if pusher == "" {
			pusher = p.Sender.display()
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
				pusher, n, word, branch, repo, repoLink),
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

	case "pull_request":
		if p.PullRequest == nil {
			return ""
		}
		pr := p.PullRequest
		action := p.Action
		if action == "closed" && pr.Merged {
			action = "merged"
		}
		return fmt.Sprintf("**%s** %s pull request [#%d %s](%s) in [%s](%s)",
			p.Sender.display(), action, pr.Number, pr.Title, pr.HTMLURL, repo, repoLink)

	case "issues":
		if p.Issue == nil {
			return ""
		}
		return fmt.Sprintf("**%s** %s issue [#%d %s](%s) in [%s](%s)",
			p.Sender.display(), p.Action, p.Issue.Number, p.Issue.Title,
			p.Issue.HTMLURL, repo, repoLink)

	case "create":
		return fmt.Sprintf("**%s** created %s **%s** in [%s](%s)",
			p.Sender.display(), p.RefType, p.Ref, repo, repoLink)

	case "delete":
		return fmt.Sprintf("**%s** deleted %s **%s** in [%s](%s)",
			p.Sender.display(), p.RefType, p.Ref, repo, repoLink)

	default:
		return ""
	}
}

// ── Card link extraction ──────────────────────────────────────────────────────

func linkGitHubCards(hook models.ProjectWebhook, event string, p githubPayload) {
	repo := p.Repository.FullName

	switch event {
	case "push":
		for _, commit := range p.Commits {
			links := services.LinkCardsFromText(commit.Message, models.CardLink{
				Platform:  "github",
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

	case "pull_request":
		if p.PullRequest == nil {
			return
		}
		pr := p.PullRequest
		status := pr.State
		if pr.Merged {
			status = "merged"
		}
		// Scan title + body (body not available in minimal struct, title is enough)
		links := services.LinkCardsFromText(pr.Title, models.CardLink{
			Platform:  "github",
			LinkType:  "pr",
			Title:     pr.Title,
			URL:       pr.HTMLURL,
			Reference: fmt.Sprintf("%d", pr.Number),
			Author:    pr.User.display(),
			Status:    status,
			RepoName:  repo,
		})
		for _, l := range links {
			appws.BroadcastToProject(hook.ProjectID, appws.Message{
				Type:    appws.TypeCardLinkCreated,
				Payload: l,
			})
		}

	case "issues":
		if p.Issue == nil {
			return
		}
		issue := p.Issue
		links := services.LinkCardsFromText(issue.Title, models.CardLink{
			Platform:  "github",
			LinkType:  "issue",
			Title:     issue.Title,
			URL:       issue.HTMLURL,
			Reference: fmt.Sprintf("%d", issue.Number),
			Author:    issue.User.display(),
			Status:    issue.State,
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

// IncomingGitHubWebhook POST /api/v1/github-webhook/:token (public)
func IncomingGitHubWebhook(c *gin.Context) {
	token := c.Param("token")

	var hook models.ProjectWebhook
	if err := database.DB.Where("token = ? AND type = ?", token, "github").First(&hook).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	sig := c.GetHeader("X-Hub-Signature-256")
	if !verifyGitHubSignature(hook.Token, body, sig) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	event := c.GetHeader("X-GitHub-Event")
	if event == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-GitHub-Event header"})
		return
	}

	// Acknowledge ping immediately.
	if event == "ping" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	var payload githubPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	// Post chat message.
	text := formatGitHubEvent(event, payload)
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
	linkGitHubCards(hook, event, payload)

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
