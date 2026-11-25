package server

import (
	"net"
	"net/http"
	"os"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/generator"
	"github.com/gin-gonic/gin"
)

type onboardingProfile struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Currency string `json:"currency"`
	Locale   string `json:"locale"`
}

var onboardingProfiles = []onboardingProfile{
	{ID: string(config.RegionalProfileGermanyEU), Label: "Deutschland/EU", Currency: "EUR", Locale: "de-DE"},
	{ID: string(config.RegionalProfileIndia), Label: "Indien", Currency: "INR", Locale: "en-IN"},
}

type onboardingRequest struct {
	Profile string `json:"profile"`
	Mode    string `json:"mode"`
}

type onboardingBackup struct {
	path       string
	backupPath string
	existed    bool
}

// HandleOnboarding serves the first-run status and initializes the
// configuration from a chosen regional profile. It is registered outside any
// auth-protected group because a fresh installation has no session yet; the
// POST is additionally guarded by the loopback check below when no user
// accounts are configured.
func HandleOnboarding(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet:
			handleOnboardingStatus(c, deps)
		case http.MethodPost:
			handleOnboardingSubmit(c, deps)
		default:
			c.JSON(http.StatusMethodNotAllowed, gin.H{"success": false, "error": "method not allowed"})
		}
	}
}

func handleOnboardingStatus(c *gin.Context, deps Deps) {
	cfg := deps.Config.Get()
	needed := deps.Config.AutoCreated() && !deps.Config.Initialized() && !cfg.Readonly

	c.JSON(http.StatusOK, gin.H{
		"needed":   needed,
		"profiles": onboardingProfiles,
	})
}

func handleOnboardingSubmit(c *gin.Context, deps Deps) {
	if deps.Config.Get().Readonly {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "configuration is read-only"})
		return
	}

	var request onboardingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}

	profile := config.RegionalProfileType(request.Profile)
	if !config.IsSupportedRegionalProfile(profile) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "unsupported regional profile"})
		return
	}

	if request.Mode != "empty" && request.Mode != "demo" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "unsupported onboarding mode"})
		return
	}

	// Without user accounts the API is unauthenticated. Restrict onboarding to
	// loopback so an open network cannot configure the installation.
	if len(deps.Config.Get().UserAccounts) == 0 && !isLoopbackOnboardingRequest(c) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "onboarding must be initiated from localhost"})
		return
	}

	if !deps.Config.TryBeginOnboarding() {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "onboarding is not available"})
		return
	}

	committed := false
	defer func() {
		if !committed {
			deps.Config.AbortOnboarding()
		}
	}()

	journalPath := deps.Config.JournalPath()
	if info, err := os.Stat(journalPath); err == nil {
		if info.Size() > 0 {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "journal already contains data"})
			return
		}
	} else if !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	configPath := deps.Config.Path()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "configuration path is not set"})
		return
	}

	backups, err := backupOnboardingFiles(configPath, journalPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	cwd := deps.Config.ConfigDir()
	if request.Mode == "demo" {
		generator.DemoForProfile(cwd, profile)
	} else {
		generator.MinimalConfigForProfile(cwd, profile)
	}

	if err := deps.Config.LoadFile(configPath); err != nil {
		restoreOnboardingBackups(backups)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	deps.Config.MarkInitialized()
	committed = true

	Sync(deps, SyncRequest{Journal: true, Prices: true, Portfolios: true})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"profile": string(profile),
		"mode":    request.Mode,
	})
}

func isLoopbackOnboardingRequest(c *gin.Context) bool {
	ip := net.ParseIP(c.ClientIP())
	return ip != nil && ip.IsLoopback()
}

// backupOnboardingFiles copies the existing configuration and journal next to
// their originals as <name>.onboarding-backup. Missing files are recorded so a
// rollback can remove anything the generator created.
func backupOnboardingFiles(paths ...string) ([]onboardingBackup, error) {
	backups := make([]onboardingBackup, 0, len(paths))

	for _, path := range paths {
		backup := onboardingBackup{path: path}

		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			backups = append(backups, backup)
			continue
		}
		if err != nil {
			restoreOnboardingBackups(backups)
			return nil, err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			restoreOnboardingBackups(backups)
			return nil, err
		}

		backup.existed = true
		backup.backupPath = path + ".onboarding-backup"
		if err := os.WriteFile(backup.backupPath, content, info.Mode().Perm()); err != nil {
			restoreOnboardingBackups(backups)
			return nil, err
		}

		backups = append(backups, backup)
	}

	return backups, nil
}

func restoreOnboardingBackups(backups []onboardingBackup) {
	for _, backup := range backups {
		if backup.existed {
			if content, err := os.ReadFile(backup.backupPath); err == nil {
				_ = os.WriteFile(backup.path, content, 0o644)
			}
			_ = os.Remove(backup.backupPath)
			continue
		}

		_ = os.Remove(backup.path)
	}
}
