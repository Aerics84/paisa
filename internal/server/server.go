package server

import (
	"context"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/auth"
	"github.com/ananthakumaran/paisa/internal/cache"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/template"
	"github.com/ananthakumaran/paisa/internal/prediction"
	"github.com/ananthakumaran/paisa/internal/pricesync"
	"github.com/ananthakumaran/paisa/internal/server/assets"
	"github.com/ananthakumaran/paisa/internal/server/goal"
	"github.com/ananthakumaran/paisa/internal/server/liabilities"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/ananthakumaran/paisa/internal/version"
	"github.com/ananthakumaran/paisa/web"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var monthlyRouteMonthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// Deps are the long-lived dependencies shared by the HTTP handlers.
type Deps struct {
	DB     *gorm.DB
	Config *config.Store
	Auth   *auth.Store
}

func Build(deps Deps, enableCompression bool) *gin.Engine {
	return BuildWithManager(deps, enableCompression, NewPriceManager(deps, pricesync.NewMemoryStateStore()))
}

func BuildWithManager(deps Deps, enableCompression bool, manager *pricesync.Manager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	if deps.Auth == nil {
		deps.Auth = auth.NewStore(sessionTTL)
	}

	db := deps.DB

	router := gin.New()
	scriptHashes, err := extractInlineScriptHashes([]byte(web.Index))
	if err != nil {
		log.Warnf("could not hash inline bootstrap script: %v", err)
	}
	router.Use(SecurityHeaders(scriptHashes))
	// Do not trust X-Forwarded-For from arbitrary clients. Otherwise a caller
	// can rotate the header to bypass the per-IP rate limiter.
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Warnf("could not disable trusted proxies: %v", err)
	}
	if enableCompression {
		router.Use(gzip.Gzip(gzip.DefaultCompression))
	}

	router.Use(Logger(log.StandardLogger()), gin.Recovery())

	router.Use(CSRFMiddleware(), TokenAuthMiddleware(deps))

	// Another process (for example `paisa update` from a timer) can change the
	// database. Drop the in-memory caches when it signals that it finished.
	router.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			cache.ClearIfStale()
		}
		c.Next()
	})

	router.GET("/robots.txt", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("User-agent: *\nDisallow: /"))
	})

	router.GET("/_app/*filepath", func(c *gin.Context) {
		c.FileFromFS("/static"+c.Request.URL.Path, http.FS(web.Static))
	})

	router.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})

	router.GET("/api/version", func(c *gin.Context) {
		c.JSON(200, gin.H{"version": version.String()})
	})

	router.POST("/api/auth/login", HandleLogin(deps))
	router.POST("/api/auth/logout", HandleLogout(deps))
	router.GET("/api/auth/status", HandleAuthStatus(deps))
	router.POST("/api/auth/hash", HandleHash(deps))

	// First-run onboarding must be reachable before a session exists. With no
	// user accounts configured the auth middleware is a no-op, and the handler
	// additionally requires a loopback request for the state-changing POST.
	router.GET("/api/onboarding", HandleOnboarding(deps))
	router.POST("/api/onboarding", HandleOnboarding(deps))

	router.GET("/api/config", func(c *gin.Context) {
		var now *time.Time
		if utils.IsNowDefined() {
			n := utils.Now().UTC()
			now = &n
		}
		accounts := accounting.AllAccounts(deps.DB)
		if accounts == nil {
			accounts = []string{}
		}
		c.JSON(200, ConfigResponse{
			Config:   deps.Config.Get(),
			Accounts: accounts,
			Now:      now,
			Schema:   config.GetSchema(),
		})
	})

	router.POST("/api/config", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, gin.H{"success": true})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		oldAccounts := deps.Config.Get().UserAccounts

		err = deps.Config.Save(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		// Changing the accounts or a password must not leave live sessions
		// authorized. Unrelated config edits keep the current sessions.
		if userAccountsChanged(oldAccounts, deps.Config.Get().UserAccounts) {
			deps.Auth.Clear()
		}

		manager.Wake()

		c.JSON(200, gin.H{"success": true})
	})

	router.GET("/api/init", HandleInit(deps))
	router.POST("/api/init", HandleInit(deps))

	router.POST("/api/sync", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, gin.H{"success": true})
			return
		}

		var syncRequest SyncRequest
		if err := c.ShouldBindJSON(&syncRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if syncRequest.Background {
			if !syncRequest.Prices || syncRequest.Journal || syncRequest.Portfolios {
				c.JSON(http.StatusBadRequest, gin.H{"error": "background is only supported for prices"})
				return
			}
			status, started := manager.Start(pricesync.SyncOptions{Full: syncRequest.Full}, pricesync.StartManual)
			c.JSON(http.StatusAccepted, gin.H{"success": true, "started": started, "status": status})
			return
		}

		c.JSON(200, Sync(deps, syncRequest))
	})

	router.GET("/api/sync/status", func(c *gin.Context) {
		c.JSON(200, manager.Status(time.Now()))
	})

	router.POST("/api/import/statement", HandleParseStatement(deps))
	router.POST("/api/import/commit", HandleCommitImport(deps))
	router.GET("/api/import/reconciliation", HandleGetReconciliation(deps))
	router.GET("/api/rules", HandleGetRules(db))
	router.POST("/api/rules", HandleSaveRule(deps))
	router.DELETE("/api/rules/:id", HandleDeleteRule(deps))

	router.GET("/api/dashboard", func(c *gin.Context) {
		c.JSON(200, GetDashboard(deps))
	})

	router.GET("/api/dashboard/monthly", func(c *gin.Context) {
		monthText := c.Query("month")
		if !monthlyRouteMonthPattern.MatchString(monthText) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "month must use YYYY-MM"})
			return
		}

		month, err := time.ParseInLocation("2006-01", monthText, deps.Config.TimeZone())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "month must use YYYY-MM"})
			return
		}

		c.JSON(200, GetMonthlyReview(deps, month))
	})

	router.GET("/api/networth", func(c *gin.Context) {
		c.JSON(200, GetNetworth(deps))
	})

	router.GET("/api/assets/balance", func(c *gin.Context) {
		c.JSON(200, assets.GetBalance(db))
	})

	router.GET("/api/investment", func(c *gin.Context) {
		c.JSON(200, GetInvestment(deps))
	})
	router.GET("/api/gain", func(c *gin.Context) {
		c.JSON(200, GetGain(deps))
	})
	router.GET("/api/gain/:account", func(c *gin.Context) {
		account := c.Param("account")
		c.JSON(200, GetAccountGain(deps, account))
	})
	router.GET("/api/income", func(c *gin.Context) {
		c.JSON(200, GetIncome(db))
	})
	router.GET("/api/expense", func(c *gin.Context) {
		c.JSON(200, GetExpense(db))
	})

	router.GET("/api/budget", func(c *gin.Context) {
		c.JSON(200, GetBudget(deps))
	})

	router.GET("/api/cash_flow", func(c *gin.Context) {
		c.JSON(200, GetCashFlow(db))
	})
	router.GET("/api/income_statement", func(c *gin.Context) {
		c.JSON(200, GetIncomeStatement(db))
	})
	router.GET("/api/recurring", func(c *gin.Context) {
		c.JSON(200, GetRecurringTransactions(db))
	})
	router.GET("/api/allocation", func(c *gin.Context) {
		c.JSON(200, GetAllocation(deps))
	})
	router.POST("/api/allocation/rebalance", RebalanceHandler(deps))
	router.POST("/api/allocation/targets", SaveAllocationTargetsHandler(deps))
	router.GET("/api/portfolio_allocation", func(c *gin.Context) {
		c.JSON(200, GetPortfolioAllocation(db))
	})
	router.GET("/api/ledger", func(c *gin.Context) {
		c.JSON(200, GetLedger(db))
	})
	router.POST("/api/price/delete", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, gin.H{"success": true})
			return
		}

		if !pricesync.DefaultRunner.TryStart() {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "price update already running"})
			return
		}
		result := ClearPriceCache(db)
		pricesync.DefaultRunner.Finish()
		c.JSON(200, result)
	})
	router.GET("/api/price", func(c *gin.Context) {
		c.JSON(200, GetPrices(deps))
	})
	router.GET("/api/price/providers", func(c *gin.Context) {
		c.JSON(200, GetPriceProviders(db))
	})

	router.POST("/api/price/providers/delete/:provider", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, gin.H{"success": true})
			return
		}

		provider := c.Param("provider")
		result, err := ClearPriceProviderCache(db, provider)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(200, result)
	})

	router.POST("/api/price/autocomplete", func(c *gin.Context) {
		var autoCompleteRequest AutoCompleteRequest
		if err := c.ShouldBindJSON(&autoCompleteRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, err := GetPriceAutoCompletions(db, autoCompleteRequest)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(200, result)
	})

	router.GET("/api/transaction/balanced", func(c *gin.Context) {
		c.JSON(200, GetBalancedPostings(db))
	})
	router.GET("/api/transaction", func(c *gin.Context) {
		c.JSON(200, GetTransactions(db))
	})
	router.GET("/api/harvest", func(c *gin.Context) {
		c.JSON(200, GetHarvest(deps))
	})

	router.GET("/api/capital_gains", func(c *gin.Context) {
		c.JSON(200, GetCapitalGains(deps))
	})

	router.GET("/api/capital_income_tax", func(c *gin.Context) {
		c.JSON(200, GetGermanyTax(deps))
	})

	router.GET("/api/tax_overview", func(c *gin.Context) {
		overview, err := GetTaxOverview(deps, c.Query("year"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, overview)
	})

	router.GET("/api/schedule_al", func(c *gin.Context) {
		c.JSON(200, GetScheduleAL(deps))
	})
	router.GET("/api/diagnosis", func(c *gin.Context) {
		c.JSON(200, GetDiagnosis(deps))
	})

	router.GET("/api/liabilities/interest", func(c *gin.Context) {
		c.JSON(200, LiabilityInterestResponse{InterestTimelineBreakdown: liabilities.GetInterest(db)})
	})

	router.GET("/api/liabilities/balance", func(c *gin.Context) {
		c.JSON(200, LiabilityBalanceResponse{LiabilityBreakdowns: liabilities.GetBalance(db)})
	})

	router.GET("/api/liabilities/repayment", func(c *gin.Context) {
		c.JSON(200, LiabilityRepaymentResponse{Repayments: liabilities.GetRepayment(db)})
	})

	router.GET("/api/logs", func(c *gin.Context) {
		c.JSON(200, GetLogs())
	})

	router.GET("/api/backup/export", HandleExportBackup(deps))

	router.GET("/api/editor/files", func(c *gin.Context) {
		c.JSON(200, GetFiles(deps))
	})

	router.POST("/api/editor/file", func(c *gin.Context) {
		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, GetFile(ledgerFile, deps.Config))
	})

	router.POST("/api/editor/file/delete_backups", func(c *gin.Context) {
		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, DeleteBackups(ledgerFile, deps.Config))
	})

	router.POST("/api/editor/validate", func(c *gin.Context) {
		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, ValidateFile(ledgerFile, deps.Config))
	})

	router.POST("/api/editor/save", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, EditorSaveResponse{Errors: []FileError{}, Saved: false, Message: "Readonly mode"})
			return
		}

		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, SaveFile(deps, ledgerFile))
	})

	router.GET("/api/sheets/files", func(c *gin.Context) {
		c.JSON(200, GetSheets(deps))
	})

	router.POST("/api/sheets/file", func(c *gin.Context) {
		var sheetFile SheetFile
		if err := c.ShouldBindJSON(&sheetFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, GetSheet(sheetFile, deps.Config))
	})

	router.POST("/api/sheets/file/delete_backups", func(c *gin.Context) {
		var sheetFile SheetFile
		if err := c.ShouldBindJSON(&sheetFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, DeleteSheetBackups(sheetFile, deps.Config))
	})

	router.POST("/api/sheets/save", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, SheetSaveResponse{Saved: false, Message: "Readonly mode"})
			return
		}

		var sheetFile SheetFile
		if err := c.ShouldBindJSON(&sheetFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, SaveSheetFile(deps, sheetFile))
	})

	router.GET("/api/account/tf_idf", func(c *gin.Context) {
		c.JSON(200, prediction.GetTfIdf(db))
	})

	router.GET("/api/templates", func(c *gin.Context) {
		c.JSON(200, TemplatesResponse{Templates: template.All()})
	})

	router.POST("/api/templates/upsert", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, gin.H{"saved": false, "message": "Readonly mode"})
			return
		}

		var t template.Template
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		saved, err := template.Upsert(t.Name, t.Content)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"saved": false, "message": err.Error()})
			return
		}

		c.JSON(200, UpsertTemplateResponse{Template: saved, Saved: true})
	})

	router.POST("/api/templates/delete", func(c *gin.Context) {
		if deps.Config.Get().Readonly {
			c.JSON(200, gin.H{"success": false, "message": "Readonly mode"})
			return
		}

		var t template.Template
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := template.Delete(t.Name); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}

		c.JSON(200, DeleteTemplateResponse{Success: true})
	})

	router.GET("/api/goals", func(c *gin.Context) {
		c.JSON(200, GoalsResponse{Goals: goal.GetGoalSummaries(db, deps.Config)})
	})

	router.GET("/api/goals/:type/:name", func(c *gin.Context) {
		c.JSON(200, goal.GetGoalDetails(db, c.Param("type"), c.Param("name"), deps.Config))
	})

	router.GET("/api/credit_cards", func(c *gin.Context) {
		c.JSON(200, GetCreditCards(deps))
	})

	router.GET("/api/credit_cards/:account", func(c *gin.Context) {
		c.JSON(200, GetCreditCard(deps, c.Param("account")))
	})

	router.NoRoute(func(c *gin.Context) {
		// SvelteKit writes non-hashed assets (manifest, service worker, icons) to
		// the root of web/static. Serve those before falling back to the SPA shell.
		requestPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if requestPath != "" {
			if data, err := fs.ReadFile(web.Static, "static/"+requestPath); err == nil {
				contentType := mime.TypeByExtension(path.Ext(requestPath))
				if strings.HasSuffix(requestPath, ".webmanifest") {
					contentType = "application/manifest+json"
				}
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				if requestPath == "sw.js" || requestPath == "manifest.webmanifest" {
					c.Header("Cache-Control", "no-cache")
				}
				c.Data(http.StatusOK, contentType, data)
				return
			}
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(web.Index))
	})

	return router
}

func Listen(deps Deps, host string, port int) {
	manager := NewPriceManager(deps, pricesync.NewFileStateStore(pricesync.StatePathForDB(deps.Config.DBPath())))
	router := BuildWithManager(deps, true, manager)
	address := net.JoinHostPort(host, strconv.Itoa(port))

	displayHost := host
	if displayHost == "" {
		displayHost = "localhost"
	}
	log.Infof("Listening on http://%s", net.JoinHostPort(displayHost, strconv.Itoa(port)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	manager.StartScheduler(ctx)

	srv := &http.Server{Addr: address, Handler: router}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	manager.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("server shutdown: %v", err)
	}
}

func rateLimitKey(c *gin.Context) string {
	return c.ClientIP()
}
