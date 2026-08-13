package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/hi2shark/santaizi-dashboard/cmd/dashboard/controller"
	"github.com/hi2shark/santaizi-dashboard/cmd/dashboard/rpc"
	"github.com/hi2shark/santaizi-dashboard/model"
	"github.com/hi2shark/santaizi-dashboard/service/availability"
	collectorservice "github.com/hi2shark/santaizi-dashboard/service/collector"
	"github.com/hi2shark/santaizi-dashboard/service/singleton"
	telemetryservice "github.com/hi2shark/santaizi-dashboard/service/telemetry"
	"github.com/ory/graceful"
	flag "github.com/spf13/pflag"
)

type DashboardCliParam struct {
	Version          bool   // 当前版本号
	ConfigFile       string // 配置文件路径
	DatebaseLocation string // Sqlite3 数据库文件路径
}

var (
	dashboardCliParam DashboardCliParam
)

func init() {
	flag.CommandLine.ParseErrorsWhitelist.UnknownFlags = true
	flag.BoolVarP(&dashboardCliParam.Version, "version", "v", false, "查看当前版本号")
	flag.StringVarP(&dashboardCliParam.ConfigFile, "config", "c", "/etc/santaizi/dashboard.yaml", "配置文件路径")
	flag.StringVar(&dashboardCliParam.DatebaseLocation, "db", "/var/lib/santaizi-dashboard/sqlite.db", "Sqlite3数据库文件路径")
	flag.Parse()
}

func initSystem() {
	// 启动 singleton 包下的所有服务
	singleton.LoadSingleton()
	availabilityEngine := availability.NewEngine(singleton.DB,
		time.Duration(singleton.Conf.Telemetry.AvailabilityBucketSeconds)*time.Second,
		singleton.Conf.Telemetry.MinObservers)
	go availabilityEngine.Run(context.Background())
	rollupWorker := telemetryservice.NewRollupWorker(singleton.DB, telemetryservice.RetentionPolicy{
		StateRaw:       time.Duration(singleton.Conf.Retention.StateRawHours) * time.Hour,
		StateOneMinute: time.Duration(singleton.Conf.Retention.StateOneMinuteDays) * 24 * time.Hour,
		StateOneHour:   time.Duration(singleton.Conf.Retention.StateOneHourDays) * 24 * time.Hour,
		Observation:    time.Duration(singleton.Conf.Retention.ObservationDays) * 24 * time.Hour,
		Lifecycle:      time.Duration(singleton.Conf.Retention.LifecycleDays) * 24 * time.Hour,
		BatchSize:      singleton.Conf.Retention.BatchSize,
	})
	go rollupWorker.Run(context.Background())
	telemetryAlerts := telemetryservice.NewAlertWorker(singleton.DB, telemetryservice.AlertPolicy{
		NotifyHostOffline:      singleton.Conf.EnableOfflineNotification,
		NotifyConnectivity:     singleton.Conf.Telemetry.EnableConnectivityNotification,
		NotifyCorrection:       singleton.Conf.Telemetry.EnableCorrectionNotification,
		NotifyCollectorOffline: singleton.Conf.Telemetry.EnableCollectorOfflineNotification,
		NotifyDataLoss:         singleton.Conf.Telemetry.EnableDataLossNotification,
		CollectorTimeout:       telemetryservice.CollectorTimeout,
	}, func(message string) { singleton.SendNotification("default", message, nil) })
	go telemetryAlerts.Run(context.Background())

	// 每天的3:30 对 监控记录 和 流量记录 进行清理
	if _, err := singleton.Cron.AddFunc("0 30 3 * * *", singleton.CleanMonitorHistory); err != nil {
		panic(err)
	}

	// 每天对超过保留期的离线历史进行清理
	if _, err := singleton.Cron.AddFunc("0 30 3 * * *", singleton.CleanOfflineHistory); err != nil {
		panic(err)
	}

	// 每小时对流量记录进行打点
	if _, err := singleton.Cron.AddFunc("0 0 * * * *", singleton.RecordTransferHourlyUsage); err != nil {
		panic(err)
	}
}

func main() {
	if dashboardCliParam.Version {
		fmt.Println(singleton.Version)
		os.Exit(0)
	}

	// 初始化 dao 包
	singleton.InitConfigFromPath(dashboardCliParam.ConfigFile)
	if singleton.Conf.Mode == "collector" {
		runCollector()
		return
	}
	singleton.InitTimezoneAndCache()
	singleton.InitDBFromPath(dashboardCliParam.DatebaseLocation)
	singleton.InitLocalizer()
	initSystem()

	// TODO 使用 cmux 在同一端口服务 HTTP 和 gRPC
	singleton.CleanMonitorHistory()
	go rpc.ServeRPC(singleton.Conf.GRPCPort)
	serviceSentinelDispatchBus := make(chan model.Monitor)
	go rpc.DispatchMonitor(serviceSentinelDispatchBus)
	go singleton.AlertSentinelStart()
	singleton.NewServiceSentinel(serviceSentinelDispatchBus)
	go singleton.StartOfflineDetector()
	srv := controller.ServeWeb(singleton.Conf.HTTPPort)
	if err := graceful.Graceful(func() error {
		return srv.ListenAndServe()
	}, func(c context.Context) error {
		log.Println("SANTAIZI>> Graceful::START")
		singleton.RecordTransferHourlyUsage()
		log.Println("SANTAIZI>> Graceful::END")
		if err := srv.Shutdown(c); err != nil {
			log.Printf("SANTAIZI>> ERROR: srv.Shutdown: %v", err)
		}
		return nil
	}); err != nil {
		log.Printf("SANTAIZI>> ERROR: %v", err)
	}
}

func runCollector() {
	store, err := collectorservice.OpenStore(singleton.Conf.Collector.DatabasePath, singleton.Conf.Debug)
	if err != nil {
		log.Fatalf("SANTAIZI>> collector database: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := collectorservice.NewRuntime(ctx, store, singleton.Conf.Collector,
		time.Duration(singleton.Conf.Telemetry.CredentialGraceDays)*24*time.Hour)
	if err != nil {
		_ = store.Close()
		log.Fatalf("SANTAIZI>> collector runtime: %v", err)
	}
	runtime.Start()
	defer func() {
		runtime.Close()
		if err := store.Close(); err != nil {
			log.Printf("SANTAIZI>> collector database close: %v", err)
		}
	}()
	if err := rpc.ServeCollectorRPC(ctx, singleton.Conf.GRPCPort, runtime); err != nil && ctx.Err() == nil {
		log.Fatalf("SANTAIZI>> collector gRPC: %v", err)
	}
}
