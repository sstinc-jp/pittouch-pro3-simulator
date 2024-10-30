package websql

import "fmt"

type Logger interface {
	//	ErrorEventf(format string, v ...interface{})

	NoticeEventf(format string, v ...interface{})

	//Criticalf(format string, v ...interface{})

	Errorf(format string, v ...interface{})

	Warningf(format string, v ...interface{})

	Debugf(flag uint32, format string, v ...interface{})
}

type EmptyLogger struct{}

//func (l *EmptyLogger) ErrorEventf(format string, v ...interface{}) {}

func (l *EmptyLogger) NoticeEventf(format string, v ...interface{}) {}

//func (l *EmptyLogger) Criticalf(format string, v ...interface{}) {}

func (l *EmptyLogger) Errorf(format string, v ...interface{}) {}

func (l *EmptyLogger) Warningf(format string, v ...interface{}) {}

func (l *EmptyLogger) Debugf(flag uint32, format string, v ...interface{}) {}

type PrintfLogger struct{}

func (l *PrintfLogger) NoticeEventf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

//func (l *PrintfLogger) Criticalf(format string, v ...interface{}) {}

func (l *PrintfLogger) Errorf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

func (l *PrintfLogger) Warningf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

func (l *PrintfLogger) Debugf(flag uint32, format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

// var websqlLog = logger.New("websql")
var websqlLog Logger = &EmptyLogger{}

func setLogger(logger Logger) {
	if logger == nil {
		websqlLog = &EmptyLogger{}
	} else {
		websqlLog = logger
	}
}
