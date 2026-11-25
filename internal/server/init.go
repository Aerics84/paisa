package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/generator"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// initMu serializes demo setup attempts. Two concurrent requests must not
// both observe an empty journal and generate over each other.
var initMu sync.Mutex

// HandleInit serves the demo-setup status and generates a demo setup. It is
// the dashboard counterpart of onboarding and deliberately shares the same
// guards: read-only installs are a no-op, and an unauthenticated install only
// accepts the request from loopback (or an explicit remote opt-in).
func HandleInit(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet:
			handleInitStatus(c, deps)
		case http.MethodPost:
			handleInitSubmit(c, deps)
		default:
			c.JSON(http.StatusMethodNotAllowed, gin.H{"success": false, "error": "method not allowed"})
		}
	}
}

func handleInitStatus(c *gin.Context, deps Deps) {
	cfg := deps.Config.Get()

	canInit := false
	reason := ""

	switch {
	case cfg.Readonly:
		reason = "configuration is read-only"
	case len(cfg.UserAccounts) == 0 && !isLoopbackOnboardingRequest(c) && !remoteOnboardingAllowed():
		reason = "demo setup must be initiated from localhost"
	default:
		hasContent, err := journalHasContent(deps.Config.JournalPath())
		switch {
		case err != nil:
			reason = err.Error()
		case hasContent:
			reason = "journal already contains data"
		default:
			canInit = true
		}
	}

	c.JSON(http.StatusOK, gin.H{"can_init": canInit, "reason": reason})
}

func handleInitSubmit(c *gin.Context, deps Deps) {
	// Preserve the historical read-only behavior: the request is a silent
	// no-op rather than an error.
	if deps.Config.Get().Readonly {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	if len(deps.Config.Get().UserAccounts) == 0 &&
		!isLoopbackOnboardingRequest(c) &&
		!remoteOnboardingAllowed() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "demo setup must be initiated from localhost"})
		return
	}

	profile := config.NormalizeRegionalProfile(deps.Config.Get().RegionalProfile)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if len(strings.TrimSpace(string(body))) > 0 {
		var request struct {
			RegionalProfile config.RegionalProfileType `json:"regional_profile"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		if request.RegionalProfile != "" {
			if !config.IsSupportedRegionalProfile(request.RegionalProfile) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("unsupported regional profile: %s", request.RegionalProfile)})
				return
			}
			profile = request.RegionalProfile
		}
	}

	initMu.Lock()
	defer initMu.Unlock()

	journalPath := deps.Config.JournalPath()
	hasContent, err := journalHasContent(journalPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if hasContent {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "journal already contains data"})
		return
	}

	configPath := deps.Config.Path()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "configuration path is not set"})
		return
	}

	configDir := deps.Config.ConfigDir()
	sheetPath := filepath.Join(configDir, "Schedule AL.paisa")

	prior := deps.Config.Get()

	backups, err := backupOnboardingFiles(configPath, journalPath, sheetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	rollback := func() {
		restoreOnboardingBackups(backups)
		rollbackInitConfig(deps, configPath, prior)
	}

	if err := generator.DemoForProfile(configDir, profile); err != nil {
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := deps.Config.LoadFile(configPath); err != nil {
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Demo generation only truncates the journal when it is still empty, so a
	// successful sync that cannot parse the generated journal means there is
	// nothing to show; undo the whole attempt.
	journalOutcome := onboardingSync(deps, SyncRequest{Journal: true})
	if success, _ := journalOutcome["success"].(bool); !success {
		message, _ := journalOutcome["message"].(string)
		if message == "" {
			message = "sync failed"
		}
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": message})
		return
	}

	var count int64
	if err := deps.DB.Model(&posting.Posting{}).Count(&count).Error; err != nil {
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if count == 0 {
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "demo sync did not seed any postings"})
		return
	}

	deps.Config.MarkInitialized()
	removeOnboardingBackups(backups)

	warning := ""
	enrichment := onboardingSync(deps, SyncRequest{Prices: true, Portfolios: true})
	if success, _ := enrichment["success"].(bool); !success {
		warning, _ = enrichment["message"].(string)
	} else {
		warning, _ = enrichment["warning"].(string)
	}

	response := gin.H{
		"success": true,
		"profile": string(profile),
		"mode":    "demo",
	}
	if warning != "" {
		response["warning"] = warning
	}
	c.JSON(http.StatusOK, response)
}

// journalHasContent reports whether the journal file exists and is non-empty.
// A missing journal is treated as empty because demo generation creates it.
func journalHasContent(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return info.Size() > 0, nil
}

// rollbackInitConfig restores the configuration that was active before the
// attempt. When the configuration file did not exist beforehand, the restored
// file system has nothing to reload, so the captured snapshot is reloaded
// directly.
func rollbackInitConfig(deps Deps, configPath string, prior config.Config) {
	if _, err := os.Stat(configPath); err == nil {
		reloadOnboardingConfig(deps, configPath)
		return
	}

	content, err := yaml.Marshal(prior)
	if err != nil {
		log.Warnf("could not serialize configuration for rollback: %v", err)
		return
	}
	if err := deps.Config.Load(content, configPath); err != nil {
		log.Warnf("could not reload configuration after failed demo setup: %v", err)
	}
}
