package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/auth"
	"github.com/ananthakumaran/paisa/internal/cache"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/generator"
	"github.com/ananthakumaran/paisa/internal/ledger"
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
	return BuildWithManager(deps, enableCompression, NewPriceManager(deps.DB, pricesync.NewMemoryStateStore()))
}

func BuildWithManager(deps Deps, enableCompression bool, manager *pricesync.Manager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	if deps.Auth == nil {
		deps.Auth = auth.NewStore(sessionTTL)
	}

	db := deps.DB

	router := gin.New()
	scriptHash, err := extractInlineScriptHash([]byte(web.Index))
	if err != nil {
		log.Warnf("could not hash inline bootstrap script: %v", err)
	}
	router.Use(SecurityHeaders(scriptHash))
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

	router.GET("/api/config", func(c *gin.Context) {
		var now *time.Time
		if utils.IsNowDefined() {
			n := utils.Now().UTC()
			now = &n
		}
		c.JSON(200, gin.H{"config": deps.Config.Get(), "accounts": accounting.AllAccounts(db), "now": now, "schema": config.GetSchema()})
	})

	router.POST("/api/config", func(c *gin.Context) {
		if config.GetConfig().Readonly {
			c.JSON(200, gin.H{"success": true})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		err = config.SaveConfig(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		manager.Wake()

		c.JSON(200, gin.H{"success": true})
	})

	router.POST("/api/init", func(c *gin.Context) {
		if config.GetConfig().Readonly {
			c.JSON(200, gin.H{"success": true})
			return
		}

		type initRequest struct {
			RegionalProfile config.RegionalProfileType `json:"regional_profile"`
		}

		profile := config.NormalizeRegionalProfile(config.GetConfig().RegionalProfile)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			var request initRequest
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

		generator.DemoForProfile(config.GetConfigDir(), profile)
		config.LoadConfigFile(config.GetConfigPath())
		Sync(db, SyncRequest{Journal: true, Prices: true, Portfolios: true})
		c.JSON(200, gin.H{"success": true})
	})

	router.POST("/api/sync", func(c *gin.Context) {
		if config.GetConfig().Readonly {
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

		c.JSON(200, Sync(db, syncRequest))
	})

	router.GET("/api/sync/status", func(c *gin.Context) {
		c.JSON(200, manager.Status(time.Now()))
	})

	router.POST("/api/import/statement", HandleParseStatement(db))
	router.POST("/api/import/commit", HandleCommitImport(db))
	router.GET("/api/import/reconciliation", HandleGetReconciliation())
	router.GET("/api/rules", HandleGetRules(db))
	router.POST("/api/rules", HandleSaveRule(db))
	router.DELETE("/api/rules/:id", HandleDeleteRule(db))

	router.GET("/api/dashboard", func(c *gin.Context) {
		c.JSON(200, GetDashboard(db))
	})

	router.GET("/api/dashboard/monthly", func(c *gin.Context) {
		monthText := c.Query("month")
		if !monthlyRouteMonthPattern.MatchString(monthText) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "month must use YYYY-MM"})
			return
		}

		month, err := time.ParseInLocation("2006-01", monthText, config.TimeZone())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "month must use YYYY-MM"})
			return
		}

		c.JSON(200, GetMonthlyReview(db, month))
	})

	router.GET("/api/networth", func(c *gin.Context) {
		c.JSON(200, GetNetworth(db))
	})

	router.GET("/api/assets/balance", func(c *gin.Context) {
		c.JSON(200, assets.GetBalance(db))
	})

	router.GET("/api/investment", func(c *gin.Context) {
		c.JSON(200, GetInvestment(db))
	})
	router.GET("/api/gain", func(c *gin.Context) {
		c.JSON(200, GetGain(db))
	})
	router.GET("/api/gain/:account", func(c *gin.Context) {
		account := c.Param("account")
		c.JSON(200, GetAccountGain(db, account))
	})
	router.GET("/api/income", func(c *gin.Context) {
		c.JSON(200, GetIncome(db))
	})
	router.GET("/api/expense", func(c *gin.Context) {
		c.JSON(200, GetExpense(db))
	})

	router.GET("/api/budget", func(c *gin.Context) {
		c.JSON(200, GetBudget(db))
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
		c.JSON(200, GetAllocation(db))
	})
	router.POST("/api/allocation/rebalance", RebalanceHandler(db))
	router.POST("/api/allocation/targets", SaveAllocationTargetsHandler())
	router.GET("/api/portfolio_allocation", func(c *gin.Context) {
		c.JSON(200, GetPortfolioAllocation(db))
	})
	router.GET("/api/ledger", func(c *gin.Context) {
		c.JSON(200, GetLedger(db))
	})
	router.POST("/api/price/delete", func(c *gin.Context) {
		if config.GetConfig().Readonly {
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
		c.JSON(200, GetPrices(db))
	})
	router.GET("/api/price/providers", func(c *gin.Context) {
		c.JSON(200, GetPriceProviders(db))
	})

	router.POST("/api/price/providers/delete/:provider", func(c *gin.Context) {
		if config.GetConfig().Readonly {
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
		c.JSON(200, GetHarvest(db))
	})

	router.GET("/api/capital_gains", func(c *gin.Context) {
		c.JSON(200, GetCapitalGains(db))
	})

	router.GET("/api/capital_income_tax", func(c *gin.Context) {
		c.JSON(200, GetGermanyTax(db))
	})

	router.GET("/api/tax_overview", func(c *gin.Context) {
		overview, err := GetTaxOverview(db, c.Query("year"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, overview)
	})

	router.GET("/api/schedule_al", func(c *gin.Context) {
		c.JSON(200, GetScheduleAL(db))
	})
	router.GET("/api/diagnosis", func(c *gin.Context) {
		c.JSON(200, GetDiagnosis(db))
	})

	router.GET("/api/liabilities/interest", func(c *gin.Context) {
		c.JSON(200, liabilities.GetInterest(db))
	})

	router.GET("/api/liabilities/balance", func(c *gin.Context) {
		c.JSON(200, liabilities.GetBalance(db))
	})

	router.GET("/api/liabilities/repayment", func(c *gin.Context) {
		c.JSON(200, liabilities.GetRepayment(db))
	})

	router.GET("/api/logs", func(c *gin.Context) {
		c.JSON(200, GetLogs())
	})

	router.GET("/api/backup/export", HandleExportBackup)

	router.GET("/api/editor/files", func(c *gin.Context) {
		c.JSON(200, GetFiles(db))
	})

	router.POST("/api/editor/file", func(c *gin.Context) {
		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, GetFile(ledgerFile))
	})

	router.POST("/api/editor/file/delete_backups", func(c *gin.Context) {
		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, DeleteBackups(ledgerFile))
	})

	router.POST("/api/editor/validate", func(c *gin.Context) {
		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, ValidateFile(ledgerFile))
	})

	router.POST("/api/editor/save", func(c *gin.Context) {
		if config.GetConfig().Readonly {
			c.JSON(200, gin.H{"errors": []ledger.LedgerFileError{}, "saved": false, "message": "Readonly mode"})
			return
		}

		var ledgerFile LedgerFile
		if err := c.ShouldBindJSON(&ledgerFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, SaveFile(db, ledgerFile))
	})

	router.GET("/api/sheets/files", func(c *gin.Context) {
		c.JSON(200, GetSheets(db))
	})

	router.POST("/api/sheets/file", func(c *gin.Context) {
		var sheetFile SheetFile
		if err := c.ShouldBindJSON(&sheetFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, GetSheet(sheetFile))
	})

	router.POST("/api/sheets/file/delete_backups", func(c *gin.Context) {
		var sheetFile SheetFile
		if err := c.ShouldBindJSON(&sheetFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, DeleteSheetBackups(sheetFile))
	})

	router.POST("/api/sheets/save", func(c *gin.Context) {
		if config.GetConfig().Readonly {
			c.JSON(200, gin.H{"saved": false, "message": "Readonly mode"})
			return
		}

		var sheetFile SheetFile
		if err := c.ShouldBindJSON(&sheetFile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, SaveSheetFile(db, sheetFile))
	})

	router.GET("/api/account/tf_idf", func(c *gin.Context) {
		c.JSON(200, prediction.GetTfIdf(db))
	})

	router.GET("/api/templates", func(c *gin.Context) {
		c.JSON(200, gin.H{"templates": template.All()})
	})

	router.POST("/api/templates/upsert", func(c *gin.Context) {
		if config.GetConfig().Readonly {
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

		c.JSON(200, gin.H{"template": saved, "saved": true})
	})

	router.POST("/api/templates/delete", func(c *gin.Context) {
		if config.GetConfig().Readonly {
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

		c.JSON(200, gin.H{"success": true})
	})

	router.GET("/api/goals", func(c *gin.Context) {
		c.JSON(200, gin.H{"goals": goal.GetGoalSummaries(db)})
	})

	router.GET("/api/goals/:type/:name", func(c *gin.Context) {
		c.JSON(200, goal.GetGoalDetails(db, c.Param("type"), c.Param("name")))
	})

	router.GET("/api/credit_cards", func(c *gin.Context) {
		c.JSON(200, GetCreditCards(db))
	})

	router.GET("/api/credit_cards/:account", func(c *gin.Context) {
		c.JSON(200, GetCreditCard(db, c.Param("account")))
	})

	router.NoRoute(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(web.Index))
	})

	return router
}

func Listen(deps Deps, host string, port int) {
	manager := NewPriceManager(deps.DB, pricesync.NewFileStateStore(pricesync.StatePathForDB(config.GetDBPath())))
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
