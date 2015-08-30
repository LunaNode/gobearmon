package gobearmon

import _ "github.com/go-sql-driver/mysql"

import "database/sql"
import "log"
import "time"

var cfg *Config

func retry(f func() error, iterations int) bool {
	for i := 0; i < iterations; i++ {
		err := f()
		if err == nil {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func Launch(cfgPath string) {
	cfg = LoadConfig(cfgPath)

	var databases []*sql.DB
	for _, dsn := range cfg.Controller.Database {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			panic(err)
		}
		databases = append(databases, db)
	}

	if cfg.Controller.Addr != "" && cfg.ViewServer.Addr != "" {
		log.Fatal("error: both controller address and viewserver address are set; you should not run both on the same instance!")
	} else if cfg.Controller.Addr != "" {
		controller := &Controller{
			Addr: cfg.Controller.Addr,
			Databases: databases,
			Confirmations: cfg.Controller.Confirmations,
		}
		controller.Start()
		worker := &Worker{
			ViewAddr: cfg.Worker.ViewAddr,
			Controller: controller,
			NumThreads: cfg.Worker.NumThreads,
		}
		worker.Start()
	} else if cfg.ViewServer.Addr != "" {
		viewServer := &ViewServer{
			Addr: cfg.ViewServer.Addr,
			Controllers: cfg.ViewServer.Controller,
		}
		viewServer.Start()
	} else {
		log.Fatal("error: neither controller address nor viewserver address is set")
	}
	select{}
}
