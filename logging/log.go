package logging

import (
	"log"
	"os"
)

var Output, _ = os.OpenFile("/var/log/process-bandwidth.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0755))

var logger = log.New(Output, "", log.LstdFlags|log.Lmicroseconds)

var pbLogger = &PBLogger{
	logger: logger,
	debug:  true,
}

type DevLogger interface {
	Logger

	Debug(v ...any)
	Debugf(format string, v ...any)
	Debugln(v ...any)

	//Info(v ...any)
	//Infof(format string, v ...any)
	//Infoln(v ...any)
	//
	//Warn(v ...any)
	//Warnf(format string, v ...any)
	//Warnln(v ...any)
	//
	//Error(v ...any)
	//Errorf(format string, v ...any)
	//Errorln(v ...any)
	//
	//Fatal(v ...any)
	//Fatalf(format string, v ...any)
	//Fatalln(v ...any)
}

type Logger interface {
	Print(v ...any)
	Printf(format string, v ...any)
	Println(v ...any)

	Fatal(v ...any)
	Fatalf(format string, v ...any)
	Fatalln(v ...any)

	Panic(v ...any)
	Panicf(format string, v ...any)
	Panicln(v ...any)
}

type PBLogger struct {
	logger Logger
	debug  bool
}

func (pbl *PBLogger) Print(v ...any) {
	pbl.logger.Print(v...)
}

func (pbl *PBLogger) Printf(format string, v ...any) {
	pbl.logger.Printf(format, v...)
}

func (pbl *PBLogger) Println(v ...any) {
	pbl.logger.Println(v...)
}

func (pbl *PBLogger) Debug(v ...any) {
	if pbl.debug {
		pbl.logger.Print(v...)
	}
}

func (pbl *PBLogger) Debugf(format string, v ...any) {
	if pbl.debug {
		pbl.logger.Printf(format, v...)
	}
}

func (pbl *PBLogger) Debugln(v ...any) {
	if pbl.debug {
		pbl.logger.Println(v...)
	}
}

func (pbl *PBLogger) Fatal(v ...any) {
	if pbl.debug {
		pbl.logger.Print(v...)
	}
}

func (pbl *PBLogger) Fatalf(format string, v ...any) {
	if pbl.debug {
		pbl.logger.Printf(format, v...)
	}
}

func (pbl *PBLogger) Fatalln(v ...any) {
	if pbl.debug {
		pbl.logger.Println(v...)
	}
}

func (pbl *PBLogger) Panic(v ...any) {
	if pbl.debug {
		pbl.logger.Print(v...)
	}
}

func (pbl *PBLogger) Panicf(format string, v ...any) {
	if pbl.debug {
		pbl.logger.Printf(format, v...)
	}
}

func (pbl *PBLogger) Panicln(v ...any) {
	if pbl.debug {
		pbl.logger.Println(v...)
	}
}

func GetLogger() DevLogger {
	return pbLogger
}
