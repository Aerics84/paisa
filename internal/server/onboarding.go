package server

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/generator"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// onboardingSync is a test seam so rollback behaviour can be exercised without
// a working ledger binary. Production code always uses Sync.
var onboardingSync = Sync

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
	mode       os.FileMode
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
	// can_setup mirrors the POST guard so a remote client can tell whether
	// finishing onboarding is actually possible instead of being redirected
	// into a loopback-only endpoint that will reject it.
	canSetup := !cfg.Readonly && (isLoopbackOnboardingRequest(c) || remoteOnboardingAllowed())

	c.JSON(http.StatusOK, gin.H{
		"needed":    needed,
		"can_setup": canSetup,
		"profiles":  onboardingProfiles,
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
	// loopback so an open network cannot configure the installation. The
	// PAISA_ALLOW_REMOTE_ONBOARDING opt-in makes the same guard reachable from
	// Docker/LAN clients that can never appear as loopback to the server.
	if len(deps.Config.Get().UserAccounts) == 0 &&
		!isLoopbackOnboardingRequest(c) &&
		!remoteOnboardingAllowed() {
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

	configDir := deps.Config.ConfigDir()
	sheetPath := filepath.Join(configDir, "Schedule AL.paisa")

	backups, err := backupOnboardingFiles(configPath, journalPath, sheetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// rollback restores the pre-onboarding files and reloads the previous
	// configuration so a failed attempt leaves a retryable installation.
	rollback := func() {
		restoreOnboardingBackups(backups)
		reloadOnboardingConfig(deps, configPath)
	}

	var generateErr error
	if request.Mode == "demo" {
		generateErr = generator.DemoForProfile(configDir, profile)
	} else {
		generateErr = generator.MinimalConfigForProfile(configDir, profile)
	}
	if generateErr != nil {
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": generateErr.Error()})
		return
	}

	if err := deps.Config.LoadFile(configPath); err != nil {
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Demo data is generated into the journal and must be parsed: without a
	// parsed journal there is nothing to show, so a missing ledger parser rolls
	// the attempt back. Empty setup only writes an empty journal, so it must not
	// require a ledger parser at all.
	if request.Mode == "demo" {
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
	}

	deps.Config.MarkInitialized()
	committed = true
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
		"mode":    request.Mode,
	}
	if warning != "" {
		response["warning"] = warning
	}
	c.JSON(http.StatusOK, response)
}

// reloadOnboardingConfig restores the store snapshot from the configuration
// file that was present before onboarding started. A missing file means the
// installation had no configuration yet; the store is left as-is and the abort
// path marks it uninitialized.
func reloadOnboardingConfig(deps Deps, configPath string) {
	if _, err := os.Stat(configPath); err != nil {
		return
	}

	if err := deps.Config.LoadFile(configPath); err != nil {
		log.Warnf("could not reload configuration after failed onboarding: %v", err)
	}
}

func isLoopbackOnboardingRequest(c *gin.Context) bool {
	ip := net.ParseIP(c.ClientIP())
	return ip != nil && ip.IsLoopback()
}

// remoteOnboardingAllowed reports whether the operator deliberately opened
// onboarding to non-loopback clients. This is intended for Docker/LAN installs
// where the browser can never reach the server through loopback; enabling it
// lets anyone who can reach the port configure the installation, so it must be
// an explicit opt-in.
func remoteOnboardingAllowed() bool {
	value := strings.TrimSpace(os.Getenv("PAISA_ALLOW_REMOTE_ONBOARDING"))
	return value == "1" || strings.EqualFold(value, "true")
}

// backupOnboardingFiles copies the existing files next to their originals as
// <name>.onboarding-backup. Missing files are recorded so a rollback can remove
// anything the generator created.
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
		backup.mode = info.Mode().Perm()
		backup.backupPath = path + ".onboarding-backup"
		if err := os.WriteFile(backup.backupPath, content, backup.mode); err != nil {
			restoreOnboardingBackups(backups)
			return nil, err
		}

		backups = append(backups, backup)
	}

	return backups, nil
}

// restoreOnboardingBackups puts the original files back. A backup is only
// deleted after its content has been written back successfully; otherwise the
// safety copy is kept so the user can recover manually.
func restoreOnboardingBackups(backups []onboardingBackup) {
	for _, backup := range backups {
		if backup.existed {
			content, err := os.ReadFile(backup.backupPath)
			if err != nil {
				log.Warnf("could not read onboarding backup %s: %v", backup.backupPath, err)
				continue
			}

			if err := os.WriteFile(backup.path, content, backup.mode); err != nil {
				log.Warnf("could not restore onboarding backup %s: %v", backup.path, err)
				continue
			}
			if backup.mode != 0 {
				if err := os.Chmod(backup.path, backup.mode); err != nil {
					log.Warnf("could not restore permissions on %s: %v", backup.path, err)
				}
			}
			if err := os.Remove(backup.backupPath); err != nil && !os.IsNotExist(err) {
				log.Warnf("could not remove onboarding backup %s: %v", backup.backupPath, err)
			}
			continue
		}

		if err := os.Remove(backup.path); err != nil && !os.IsNotExist(err) {
			log.Warnf("could not remove generated onboarding file %s: %v", backup.path, err)
		}
	}
}

// removeOnboardingBackups discards the safety copies after a successful
// onboarding run.
func removeOnboardingBackups(backups []onboardingBackup) {
	for _, backup := range backups {
		if !backup.existed {
			continue
		}
		if err := os.Remove(backup.backupPath); err != nil && !os.IsNotExist(err) {
			log.Warnf("could not remove onboarding backup %s: %v", backup.backupPath, err)
		}
	}
}
