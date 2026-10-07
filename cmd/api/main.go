// Command api is the HTTP entrypoint for the HRMS backend. It wires every
// dependency by hand and shuts down gracefully on SIGINT or SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/config"
	httpdelivery "github.com/fauzanmzlm/hrms-go-backend/internal/delivery/http"
	"github.com/fauzanmzlm/hrms-go-backend/internal/delivery/http/handler"
	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/internal/repository/mysql"
	"github.com/fauzanmzlm/hrms-go-backend/internal/usecase"
	"github.com/fauzanmzlm/hrms-go-backend/migrations"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/jwt"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/logger"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/password"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := mysql.Open(ctx, mysql.Config{
		DSN:             cfg.DBDSN,
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("closing database", slog.String("error", err.Error()))
			return
		}
		log.Info("database connection closed")
	}()
	log.Info("database connected")

	if cfg.AutoMigrate {
		if err := mysql.Migrate(ctx, db, migrations.FS, log); err != nil {
			return err
		}
	}

	app, err := buildApp(cfg, db, log)
	if err != nil {
		return err
	}
	if err := bootstrapAdmin(ctx, cfg, app.auth, log); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           app.handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	// Restore default signal handling so a second Ctrl+C forces exit.
	stop()
	log.Info("shutdown signal received, draining connections", slog.String("timeout", cfg.ShutdownTimeout.String()))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}
	log.Info("http server stopped")
	return nil
}

type app struct {
	handler http.Handler
	auth    domain.AuthUsecase
}

// buildApp is the composition root: repositories, then usecases, then
// handlers and the router.
func buildApp(cfg *config.Config, db *sqlx.DB, log *slog.Logger) (*app, error) {
	tx := mysql.NewTransactor(db)
	grades := mysql.NewGradeRepository(db)
	employees := mysql.NewEmployeeRepository(db)
	records := mysql.NewServiceRecordRepository(db)
	promotions := mysql.NewPromotionRepository(db)
	disciplinary := mysql.NewDisciplinaryRepository(db)
	users := mysql.NewUserRepository(db)

	tokens, err := jwt.NewManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	if err != nil {
		return nil, err
	}
	hasher := password.NewBcrypt(cfg.BcryptCost)

	authUC, err := usecase.NewAuthUsecase(users, employees, hasher, tokens)
	if err != nil {
		return nil, err
	}
	gradeUC := usecase.NewGradeUsecase(grades)
	employeeUC := usecase.NewEmployeeUsecase(employees, grades)
	recordUC := usecase.NewServiceRecordUsecase(tx, employees, grades, records)
	promotionUC := usecase.NewPromotionUsecase(tx, employees, grades, records, promotions, disciplinary, users,
		usecase.PromotionConfig{DisciplinaryBlockMonths: cfg.PromotionDisciplinaryBlockMonths}, usecase.SystemClock)
	disciplinaryUC := usecase.NewDisciplinaryUsecase(tx, employees, disciplinary, users, usecase.SystemClock)

	router := httpdelivery.NewRouter(httpdelivery.RouterDeps{
		Logger: log,
		Tokens: tokens,
		DB:     db,
		Handlers: httpdelivery.Handlers{
			Auth:          handler.NewAuthHandler(authUC, log),
			Grade:         handler.NewGradeHandler(gradeUC, log),
			Employee:      handler.NewEmployeeHandler(employeeUC, log),
			ServiceRecord: handler.NewServiceRecordHandler(recordUC, log),
			Promotion:     handler.NewPromotionHandler(promotionUC, log),
			Disciplinary:  handler.NewDisciplinaryHandler(disciplinaryUC, log),
		},
	})
	return &app{handler: router, auth: authUC}, nil
}

// bootstrapAdmin creates the configured ADMIN account on first start so a
// fresh database can be logged into. An existing account is left untouched.
func bootstrapAdmin(ctx context.Context, cfg *config.Config, auth domain.AuthUsecase, log *slog.Logger) error {
	if cfg.BootstrapAdminUsername == "" {
		return nil
	}
	u := &domain.User{Username: cfg.BootstrapAdminUsername, Role: domain.RoleAdmin}
	err := auth.CreateUser(ctx, u, cfg.BootstrapAdminPassword)
	switch {
	case err == nil:
		log.Info("bootstrap admin created", slog.String("username", u.Username))
	case errors.Is(err, domain.ErrConflict):
		log.Debug("bootstrap admin already exists", slog.String("username", u.Username))
	default:
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	return nil
}
