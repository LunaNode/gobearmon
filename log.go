package gobearmon

import "log"

func debugPrintf(format string, v ...interface{}) {
	if cfg.Default.Debug {
		log.Printf(format, v...)
	}
}
